/*
 *  Copyright 2023 The Cerebro Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
package broker

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/risk"
)

// Broker tracks cash and open orders. Cash accounting is exchange-authoritative:
// balance reflects the settled cash reported by the market, while open buy orders
// reserve cash so the broker never over-commits before settlement.
type Broker struct {
	EventEngine event.Broadcaster
	market      market.Market
	logger      log.Logger
	// orders holds open (unsettled) orders. An order leaves this set once it is
	// completed, canceled, or rejected, which releases its cash reservation.
	orders []order.Order
	// positions is the latest snapshot of account positions from the market.
	positions []position.Position
	// balance is the settled cash reported by the exchange. It is seeded from
	// AccountBalance() and updated from market.ChangeBalanceEvent.
	balance decimal.Decimal
	// lots tracks each strategy's open long position per code, rebuilt from the
	// attributed fills the broker observes. size is the held quantity and cost is
	// the price*size acquisition cost (commission excluded), so the average entry
	// is cost/size. It is the source of truth for realized PnL and reporting.
	lots map[string]map[string]*lot
	// realized accumulates each strategy's realized PnL (price gains/losses on
	// closed quantity); fees accumulates the commission it has paid. Net realized
	// performance is realized minus fees.
	realized map[string]decimal.Decimal
	fees     map[string]decimal.Decimal
	// filled tracks cumulative filled size per order id, so a partial-then-complete
	// sequence books each increment once. It is keyed off the immutable order Size,
	// not the order's remaining size, which a market adapter may mutate on the
	// shared order object before the broker observes the fill event.
	filled map[string]decimal.Decimal
	// lastPrice is the most recent tick price per code. It is the final fallback for
	// valuing a market fill whose event and order both carry no price, so positions
	// (and the PnL/risk that read them) are still tracked for such adapters.
	lastPrice map[string]decimal.Decimal
	// mu guards orders, positions, balance, and the PnL ledger (lots/realized/fees).
	mu sync.RWMutex
	// wg tracks in-flight submit goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
	// risk is the optional pre-trade gate consulted in Order; nil means no gate.
	risk *risk.Manager
}

// lot is one strategy's open position in one code.
type lot struct {
	item *item.Item
	size decimal.Decimal // held quantity
	cost decimal.Decimal // acquisition cost of the held quantity (price*size, no fees)
	peak decimal.Decimal // highest fill price since the lot opened (seeds trailing stops)
}

// StrategyReport is a per-strategy view of trading performance at a point in time.
type StrategyReport struct {
	Strategy  string
	Realized  decimal.Decimal // realized PnL (price gains/losses on closed quantity)
	Fees      decimal.Decimal // cumulative commission paid
	Positions []position.Position
}

// SetRisk installs the pre-trade risk gate. It is set once at construction before
// any order flows, so it needs no synchronization.
func (b *Broker) SetRisk(rm *risk.Manager) { b.risk = rm }

// snapshotLocked captures the account state the risk rules vet an order against.
// Callers must hold b.mu so the snapshot (including pending orders) is consistent
// with the reservation done in the same critical section.
func (b *Broker) snapshotLocked() risk.Snapshot {
	return risk.Snapshot{
		Balance:   b.balance,
		Available: b.balance.Sub(b.reservedLocked()),
		Realized:  b.totalRealizedLocked(),
		Positions: slices.Clone(b.positions),
		Open:      slices.Clone(b.orders),
	}
}

var hundred = decimal.NewFromInt(100)

// recordFillLocked folds one fill into the per-strategy PnL ledger. Caller holds
// b.mu. A buy grows the lot; a sell realizes PnL on the closed quantity at the
// lot's average cost and shrinks (or clears) it. Commission is a percentage, so
// the fee is value * commission / 100.
func (b *Broker) recordFillLocked(strategy string, it *item.Item, action order.Action, size, price decimal.Decimal) {
	lots := b.lots[strategy]
	if lots == nil {
		lots = map[string]*lot{}
		b.lots[strategy] = lots
	}
	l := lots[it.Code]
	if l == nil {
		l = &lot{item: it}
		lots[it.Code] = l
	}
	commission := b.market.Commission()

	switch action {
	case order.Buy:
		b.fees[strategy] = b.fees[strategy].Add(price.Mul(size).Mul(commission).Div(hundred))
		l.cost = l.cost.Add(price.Mul(size))
		l.size = l.size.Add(size)
		if price.GreaterThan(l.peak) {
			l.peak = price // track the high-water fill price for trailing stops
		}
	case order.Sell:
		// Never close more than is held (a defensive guard; the exchange/replay
		// should not report an oversell).
		sold := decimal.Min(size, l.size)
		if sold.GreaterThan(decimal.Zero) {
			avg := l.cost.Div(l.size)
			b.fees[strategy] = b.fees[strategy].Add(price.Mul(sold).Mul(commission).Div(hundred))
			b.realized[strategy] = b.realized[strategy].Add(price.Sub(avg).Mul(sold))
			l.cost = l.cost.Sub(avg.Mul(sold))
			l.size = l.size.Sub(sold)
		}
		if l.size.LessThanOrEqual(decimal.Zero) {
			delete(lots, it.Code)
			if len(lots) == 0 {
				delete(b.lots, strategy)
			}
		}
	default:
		// Cancel/Edit are not fills and do not move the position.
	}
}

// totalRealizedLocked sums realized PnL across strategies. Caller holds b.mu.
func (b *Broker) totalRealizedLocked() decimal.Decimal {
	sum := decimal.Zero
	for _, v := range b.realized {
		sum = sum.Add(v)
	}
	return sum
}

// StrategyPosition returns a strategy's open position in a code (average entry as
// Position.Price), the high-water fill price since it opened, and ok=false if it
// holds none. It is the source of truth the reactive risk monitor evaluates exit
// policies against; the fill high lets a trailing stop account for scale-ins above
// the average entry that no observed tick may have captured.
func (b *Broker) StrategyPosition(strategy, code string) (position.Position, decimal.Decimal, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	l := b.lots[strategy][code]
	if l == nil || l.size.LessThanOrEqual(decimal.Zero) {
		return position.Position{}, decimal.Zero, false
	}
	return position.Position{Item: l.item, Size: l.size, Price: l.cost.Div(l.size)}, l.peak, true
}

// RealizedPnL returns the realized PnL accumulated by one strategy.
func (b *Broker) RealizedPnL(strategy string) decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.realized[strategy]
}

// TotalRealizedPnL returns realized PnL across every strategy.
func (b *Broker) TotalRealizedPnL() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.totalRealizedLocked()
}

// Report returns a per-strategy snapshot of realized PnL, fees, and open
// positions, sorted by strategy name for stable output.
func (b *Broker) Report() []StrategyReport {
	b.mu.RLock()
	defer b.mu.RUnlock()

	names := map[string]struct{}{}
	for s := range b.realized {
		names[s] = struct{}{}
	}
	for s := range b.fees {
		names[s] = struct{}{}
	}
	for s := range b.lots {
		names[s] = struct{}{}
	}

	out := make([]StrategyReport, 0, len(names))
	for s := range names {
		rep := StrategyReport{Strategy: s, Realized: b.realized[s], Fees: b.fees[s]}
		for _, l := range b.lots[s] {
			avg := decimal.Zero
			if l.size.GreaterThan(decimal.Zero) {
				avg = l.cost.Div(l.size)
			}
			rep.Positions = append(rep.Positions, position.Position{Item: l.item, Size: l.size, Price: avg})
		}
		sort.Slice(rep.Positions, func(i, j int) bool {
			return rep.Positions[i].Item.Code < rep.Positions[j].Item.Code
		})
		out = append(out, rep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Strategy < out[j].Strategy })
	return out
}

// Submitter is the broker surface a strategy uses. A scoped Submitter tags every
// order with the strategy's name so fills can be attributed back to it.
type Submitter interface {
	Order(ctx context.Context, o order.Order, safe bool) error
	Available() decimal.Decimal
	Balance() decimal.Decimal
	Position(ticker string) (position.Position, bool)
	Orders(code string) []order.Order
}

var (
	_ Submitter = (*Broker)(nil)
	_ risk.Book = (*Broker)(nil)
)

// Scoped returns a Submitter that stamps strategy onto every order before
// submitting it through the broker.
func (b *Broker) Scoped(strategy string) Submitter {
	return &scopedBroker{Broker: b, strategy: strategy}
}

type scopedBroker struct {
	*Broker
	strategy string
}

func (s *scopedBroker) Order(ctx context.Context, o order.Order, safe bool) error {
	o.SetStrategy(s.strategy)
	return s.Broker.Order(ctx, o, safe)
}

func NewDefaultBroker(eventEngine event.Broadcaster, store market.Market, logger log.Logger) *Broker {
	return &Broker{
		orders:      []order.Order{},
		EventEngine: eventEngine,
		market:      store,
		logger:      logger,
		positions:   store.AccountPositions(),
		balance:     store.AccountBalance(),
		lots:        map[string]map[string]*lot{},
		realized:    map[string]decimal.Decimal{},
		fees:        map[string]decimal.Decimal{},
		filled:      map[string]decimal.Decimal{},
		lastPrice:   map[string]decimal.Decimal{},
	}
}

// orderValue returns the cash an open order still commits, including commission.
// It is based on the unfilled remainder (RemainPrice), so a partial fill releases
// the reservation for the portion that has already settled. For a fresh order the
// remainder equals the full size. Commission() is a percentage, so the fee is
// value * commission / 100.
func (b *Broker) orderValue(o order.Order) decimal.Decimal {
	value := o.RemainPrice()
	fee := value.Mul(b.market.Commission()).Div(decimal.NewFromInt(100))
	return value.Add(fee)
}

// isTerminalStatus reports whether a status ends an order's lifecycle, meaning
// the order should leave the open set and release any cash it reserved.
func isTerminalStatus(s order.Status) bool {
	switch s {
	case order.Completed, order.Canceled, order.Expired, order.Margin, order.Rejected:
		return true
	default:
		return false
	}
}

// reservedLocked returns the cash committed by open buy orders.
// Callers must hold b.mu (read or write).
func (b *Broker) reservedLocked() decimal.Decimal {
	reserved := decimal.Zero
	for i := range b.orders {
		if b.orders[i].Action() == order.Buy {
			reserved = reserved.Add(b.orderValue(b.orders[i]))
		}
	}
	return reserved
}

// Balance returns the settled cash reported by the exchange.
func (b *Broker) Balance() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.balance
}

// Available returns the cash available for new buy orders:
// settled balance minus cash reserved by open buy orders.
func (b *Broker) Available() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.balance.Sub(b.reservedLocked())
}

func (b *Broker) Order(ctx context.Context, o order.Order, safe bool) error {
	if o.Type() == order.Market && !o.Price().IsZero() {
		b.logger.Error("invalid order price", "code", o.Item().Code, "price", o.Price(), "size", o.Size())
		return fmt.Errorf("invalid order price, market order price must be set 0")
	}

	if o.Type() == order.Limit && o.Size().IsZero() {
		b.logger.Error("invalid order size", "code", o.Item().Code, "price", o.Price(), "size", o.Size())
		return ErrOrderSizeIsZero
	}

	if o.Type() == order.Limit && o.Price().IsZero() {
		b.logger.Error("invalid order price", "code", o.Item().Code, "price", o.Price(), "size", o.Size())
		return ErrPriceIsZero
	}

	b.mu.Lock()
	// Pre-trade risk gate: vet against the current account state — including
	// pending orders — atomically with the reservation below, before submitting.
	if b.risk != nil {
		if err := b.risk.Check(o, b.snapshotLocked()); err != nil {
			b.mu.Unlock()
			b.logger.Info("order rejected by risk gate", "code", o.Item().Code, "error", err)
			return err
		}
	}
	if safe {
		if slices.ContainsFunc(b.orders, func(od order.Order) bool {
			return od.Item().Code == o.Item().Code
		}) {
			b.mu.Unlock()
			return errors.New("waiting for conclusion")
		}
	}

	// Reserve cash for buy orders against available (settled minus reserved) cash.
	if o.Action() == order.Buy {
		if value := b.orderValue(o); value.GreaterThan(b.balance.Sub(b.reservedLocked())) {
			b.mu.Unlock()
			return ErrNotEnoughMoney
		}
	}

	b.orders = append(b.orders, o)
	b.mu.Unlock()

	b.wg.Go(func() {
		b.submit(ctx, o)
	})
	return nil
}

// Wait blocks until all in-flight order submissions have completed. Callers must
// keep the event dispatcher alive until Wait returns, since submit broadcasts.
func (b *Broker) Wait() {
	b.wg.Wait()
}

// submit sends the order to the market in a goroutine. Cash accounting follows
// the exchange: balance is updated from ChangeBalanceEvent and an order's
// reservation is released once it leaves b.orders (complete/cancel/reject).
func (b *Broker) submit(ctx context.Context, o order.Order) {
	o.Submit()
	b.notifyOrder(ctx, o.Copy())

	if err := b.market.Order(ctx, o); err != nil {
		b.logger.Info("reject order", "order", o, "error", err)
		o.Reject()
		b.removeOrder(o)
		b.notifyOrder(ctx, o.Copy())
	}
}

func (b *Broker) notifyOrder(ctx context.Context, o order.Order) {
	// Drop the notification if shutdown is underway; the dispatcher may be draining.
	b.EventEngine.BroadCastContext(ctx, o)
}

// removeOrder drops an order from the open set, releasing its cash reservation.
func (b *Broker) removeOrder(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.orders = slices.DeleteFunc(b.orders, func(od order.Order) bool {
		return od.ID() == o.ID()
	})
}

func (b *Broker) Orders(code string) []order.Order {
	b.mu.RLock()
	defer b.mu.RUnlock()

	orders := []order.Order{}
	for i := range b.orders {
		if b.orders[i].Item().Code == code {
			orders = append(orders, b.orders[i])
		}
	}
	return orders
}

func (b *Broker) Position(ticker string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return lo.Find(b.positions, func(item position.Position) bool {
		return item.Item.Code == ticker
	})
}

// refreshPositions pulls the latest positions from the market. The market call
// runs outside the lock to keep the critical section short.
func (b *Broker) refreshPositions() {
	positions := b.market.AccountPositions()
	b.mu.Lock()
	b.positions = positions
	b.mu.Unlock()
}

func (b *Broker) Listen(ctx context.Context, e any) {
	switch evt := e.(type) {
	case order.Order:
		if evt.Status() == order.Submitted {
			b.refreshPositions()
		}
	case indicator.Tick:
		// Remember the latest price per code so a market fill that carries no price
		// can still be valued (see applyOrderChange).
		b.mu.Lock()
		b.lastPrice[evt.Code] = evt.Price
		b.mu.Unlock()
	case market.MarketEvent:
		b.handleMarketEvent(ctx, evt)
	}
}

func (b *Broker) handleMarketEvent(ctx context.Context, m market.MarketEvent) {
	switch evt := m.(type) {
	case market.ChangeOrderEvent:
		b.logger.Info("market change order", "message", evt.Message, "id", evt.ID, "action", evt.Action)
		b.applyOrderChange(ctx, evt)
	case market.ChangeBalanceEvent:
		b.logger.Info("market change balance", "message", evt.Message, "balance", evt.Balance)
		b.mu.Lock()
		b.balance = evt.Balance
		b.mu.Unlock()
	}
}

// applyOrderChange updates an open order from an exchange event. Completed and
// canceled orders leave the open set, releasing their cash reservation.
func (b *Broker) applyOrderChange(ctx context.Context, evt market.ChangeOrderEvent) {
	// Pull the latest positions outside the lock (the market call is slow), then
	// apply the order-state change and the position refresh under one lock. Doing
	// both atomically means a fill's exposure is never momentarily absent from
	// both the open set and positions, which would let the risk gate under-count
	// it if a strategy places an order in reaction to the fill notification.
	positions := b.market.AccountPositions()

	b.mu.Lock()
	idx := slices.IndexFunc(b.orders, func(od order.Order) bool {
		return od.ID() == evt.ID
	})
	if idx < 0 {
		b.mu.Unlock()
		return
	}

	o := b.orders[idx]
	// Capture the size newly filled by this event before mutating the order, so the
	// PnL ledger counts each increment exactly once (a partial then a completion add
	// up to the full size).
	var fillInc decimal.Decimal
	switch evt.Action {
	case order.Accepted:
		o.Accept()
	case order.Completed:
		// Completion fills whatever remains; derive it from the immutable order size
		// minus what already filled, not from the (possibly externally mutated)
		// remaining size.
		fillInc = o.Size().Sub(b.filled[evt.ID])
		o.Complete()
	case order.Canceled:
		o.Cancel()
	case order.Expired:
		o.Expire()
	case order.Margin:
		o.Margin()
	case order.Rejected:
		o.Reject()
	case order.Partial:
		// A partial fill reduces the remaining size; the order stays open and its
		// reservation shrinks to the unfilled remainder (see orderValue).
		fillInc = evt.FilledSize
		b.filled[evt.ID] = b.filled[evt.ID].Add(evt.FilledSize)
		o.Partial(evt.FilledSize)
	default:
		// None/Created/Submitted are not delivered as exchange events; nothing to apply.
	}
	// Fold the fill into the per-strategy PnL ledger. The exchange's fill price is
	// preferred; for limit orders the order's own price is an exact fallback.
	if fillInc.GreaterThan(decimal.Zero) {
		price := evt.Price
		if price.IsZero() {
			price = o.Price() // limit orders carry an exact price
		}
		if price.IsZero() {
			price = b.lastPrice[o.Item().Code] // market fill with no reported price
		}
		if price.GreaterThan(decimal.Zero) {
			b.recordFillLocked(o.Strategy(), o.Item(), o.Action(), fillInc, price)
		}
	}
	// Terminal statuses end the order's lifecycle: drop it from the open set so
	// its reserved cash is released. Non-terminal updates (e.g. Accepted) keep
	// the order open and its reservation in place.
	if isTerminalStatus(evt.Action) {
		b.orders = slices.Delete(b.orders, idx, idx+1)
		delete(b.filled, evt.ID)
	}
	b.positions = positions
	b.mu.Unlock()

	b.logger.Info("order changed", "id", o.ID(), "code", o.Item().Code, "action", evt.Action)
	b.notifyOrder(ctx, o.Copy())
}
