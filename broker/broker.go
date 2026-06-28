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
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
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
	logger      *slog.Logger
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
	// now is the latest observed tick time, the clock the broker stamps trades and
	// equity samples with (sim time under replay, wall time live). Zero until the
	// first tick.
	now time.Time
	// trades is the closed round-trip log, appended when a lot returns to flat, the
	// basis for trade-level performance metrics (win rate, profit factor, ...).
	trades []Trade
	// equity is the account equity sampled once per calendar day (by tick time), the
	// basis for time-level metrics (max drawdown, Sharpe, total return).
	equity []EquityPoint
	// mu guards orders, positions, balance, and the PnL ledger (lots/realized/fees).
	mu sync.RWMutex
	// wg tracks in-flight submit goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
	// risk is the optional pre-trade gate consulted in Order; nil means no gate.
	risk *risk.Manager
	// store is the optional durable ledger persisted after each booked fill and
	// reloaded on Restore; nil means no persistence (existing behavior).
	store Storage
	// saveMu serializes ledger writes through store so overlapping saves cannot
	// interleave or move the persisted state backwards.
	saveMu sync.Mutex
}

// lot is one strategy's open position in one code. The round-trip accumulators below
// span the lot's life (first buy to flat) so a Trade can be emitted when it closes;
// they are distinct from size/cost, which track only the currently-held quantity.
type lot struct {
	item *item.Item
	size decimal.Decimal // held quantity
	cost decimal.Decimal // acquisition cost of the held quantity (price*size, no fees)
	peak decimal.Decimal // highest fill price since the lot opened (seeds trailing stops)

	openedAt    time.Time       // time of the first buy that opened this round-trip
	lastAt      time.Time       // time of the most recent fill (becomes Trade.ClosedAt)
	boughtQty   decimal.Decimal // total quantity bought over the round-trip
	boughtValue decimal.Decimal // total buy notional (price*size) — avg entry is value/qty
	soldQty     decimal.Decimal // total quantity sold over the round-trip
	soldValue   decimal.Decimal // total sell notional — avg exit is value/qty
	realized    decimal.Decimal // round-trip realized PnL (price gains on closed quantity)
	feesPaid    decimal.Decimal // round-trip commission (buys + sells)
}

// Trade is one completed round-trip: a position opened from flat and returned to
// flat. NetPnL is Realized minus Fees; a winning trade has NetPnL > 0.
type Trade struct {
	Strategy string
	Code     string
	Qty      decimal.Decimal // quantity traded (entry size)
	Entry    decimal.Decimal // average entry price
	Exit     decimal.Decimal // average exit price
	Realized decimal.Decimal // gross realized PnL over the round-trip
	Fees     decimal.Decimal // commission over the round-trip
	OpenedAt time.Time
	ClosedAt time.Time
}

// NetPnL is the round-trip profit after commission.
func (t Trade) NetPnL() decimal.Decimal { return t.Realized.Sub(t.Fees) }

// EquityPoint is account equity (settled cash plus the mark-to-market value of open
// positions) sampled at a point in time.
type EquityPoint struct {
	Time   time.Time
	Equity decimal.Decimal
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

// recordFillLocked folds one fill into the per-strategy PnL ledger. Caller holds
// b.mu. A buy grows the lot; a sell realizes PnL on the closed quantity at the
// lot's average cost and shrinks (or clears) it. The commission Rate carries its
// own unit, so the fee is rate.Of(value).
func (b *Broker) recordFillLocked(strategy string, it *item.Item, action order.Action, size, price decimal.Decimal) {
	lots := b.lots[strategy]
	if lots == nil {
		lots = map[string]*lot{}
		b.lots[strategy] = lots
	}
	l := lots[it.Code]
	if l == nil {
		l = &lot{item: it, openedAt: b.now}
		lots[it.Code] = l
	}
	l.lastAt = b.now
	rate := b.market.Commission()

	switch action {
	case order.Buy:
		fee := rate.Of(price.Mul(size))
		b.fees[strategy] = b.fees[strategy].Add(fee)
		l.feesPaid = l.feesPaid.Add(fee)
		l.cost = l.cost.Add(price.Mul(size))
		l.size = l.size.Add(size)
		l.boughtQty = l.boughtQty.Add(size)
		l.boughtValue = l.boughtValue.Add(price.Mul(size))
		if price.GreaterThan(l.peak) {
			l.peak = price // track the high-water fill price for trailing stops
		}
	case order.Sell:
		// Never close more than is held (a defensive guard; the exchange/replay
		// should not report an oversell).
		sold := decimal.Min(size, l.size)
		if sold.GreaterThan(decimal.Zero) {
			avg := l.cost.Div(l.size)
			fee := rate.Of(price.Mul(sold))
			gain := price.Sub(avg).Mul(sold)
			b.fees[strategy] = b.fees[strategy].Add(fee)
			b.realized[strategy] = b.realized[strategy].Add(gain)
			l.feesPaid = l.feesPaid.Add(fee)
			l.realized = l.realized.Add(gain)
			l.soldQty = l.soldQty.Add(sold)
			l.soldValue = l.soldValue.Add(price.Mul(sold))
			l.cost = l.cost.Sub(avg.Mul(sold))
			l.size = l.size.Sub(sold)
		}
		if l.size.LessThanOrEqual(decimal.Zero) {
			b.trades = append(b.trades, tradeFromLot(strategy, l))
			delete(lots, it.Code)
			if len(lots) == 0 {
				delete(b.lots, strategy)
			}
		}
	default:
		// Cancel/Edit are not fills and do not move the position.
	}
}

// tradeFromLot builds the closed-trade record for a lot that has returned to flat.
func tradeFromLot(strategy string, l *lot) Trade {
	t := Trade{
		Strategy: strategy,
		Code:     l.item.Code,
		Qty:      l.boughtQty,
		Realized: l.realized,
		Fees:     l.feesPaid,
		OpenedAt: l.openedAt,
		ClosedAt: l.lastAt,
	}
	if l.boughtQty.GreaterThan(decimal.Zero) {
		t.Entry = l.boughtValue.Div(l.boughtQty)
	}
	if l.soldQty.GreaterThan(decimal.Zero) {
		t.Exit = l.soldValue.Div(l.soldQty)
	}
	return t
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

// equityLocked is settled cash plus the mark-to-market value of every open lot,
// valued at the latest tick price (falling back to average entry when a held code
// has no observed price yet). Caller holds b.mu.
func (b *Broker) equityLocked() decimal.Decimal {
	eq := b.balance
	for _, lots := range b.lots {
		for code, l := range lots {
			if l.size.LessThanOrEqual(decimal.Zero) {
				continue
			}
			px, ok := b.lastPrice[code]
			if !ok {
				px = l.cost.Div(l.size) // no tick yet: mark at entry (unrealized 0)
			}
			eq = eq.Add(px.Mul(l.size))
		}
	}
	return eq
}

// sampleEquityLocked keeps one equity point per calendar day (in tick time): it
// appends a point on a new day and otherwise updates the current day's point to the
// latest equity, so each completed day holds that day's closing equity (reflecting
// intraday price moves and fills) rather than its open. Caller holds b.mu.
func (b *Broker) sampleEquityLocked(now time.Time) {
	eq := b.equityLocked()
	if n := len(b.equity); n > 0 && sameDay(b.equity[n-1].Time, now) {
		b.equity[n-1] = EquityPoint{Time: now, Equity: eq}
		return
	}
	b.equity = append(b.equity, EquityPoint{Time: now, Equity: eq})
}

// sameDay reports whether a and b fall on the same UTC calendar day.
func sameDay(a, c time.Time) bool {
	ay, am, ad := a.UTC().Date()
	cy, cm, cd := c.UTC().Date()
	return ay == cy && am == cm && ad == cd
}

// Equity returns the current account equity (settled cash plus mark-to-market open
// positions).
func (b *Broker) Equity() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.equityLocked()
}

// Trades returns a copy of the closed round-trip log, oldest first.
func (b *Broker) Trades() []Trade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return slices.Clone(b.trades)
}

// EquityCurve returns the daily-sampled equity series, oldest first, with the final
// (current) day reflecting equity as of now — including fills booked since the last
// tick — so the curve always ends at the live equity.
func (b *Broker) EquityCurve() []EquityPoint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	curve := slices.Clone(b.equity)
	if b.now.IsZero() {
		return curve
	}
	cur := b.equityLocked()
	if n := len(curve); n > 0 && sameDay(curve[n-1].Time, b.now) {
		curve[n-1] = EquityPoint{Time: b.now, Equity: cur}
	} else {
		curve = append(curve, EquityPoint{Time: b.now, Equity: cur})
	}
	return curve
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

func NewDefaultBroker(eventEngine event.Broadcaster, store market.Market, logger *slog.Logger) *Broker {
	return &Broker{
		orders:      []order.Order{},
		EventEngine: eventEngine,
		market:      store,
		logger:      logger,
		// Seed settled cash and positions from the exchange at startup. There is no
		// request context here, so use Background; the adapter must keep these reads
		// time-bounded.
		positions: store.AccountPositions(context.Background()),
		balance:   store.AccountBalance(context.Background()),
		lots:      map[string]map[string]*lot{},
		realized:  map[string]decimal.Decimal{},
		fees:      map[string]decimal.Decimal{},
		filled:    map[string]decimal.Decimal{},
		lastPrice: map[string]decimal.Decimal{},
	}
}

// orderValue returns the cash an open order still commits, including commission.
// It is based on the unfilled remainder (RemainPrice), so a partial fill releases
// the reservation for the portion that has already settled. For a fresh order the
// remainder equals the full size. The commission Rate carries its own unit, so
// the fee is Commission().Of(value).
func (b *Broker) orderValue(o order.Order) decimal.Decimal {
	value := o.RemainPrice()
	fee := b.market.Commission().Of(value)
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
	// Pre-trade validation: return the error to the caller as the single handling
	// point — the broker does not also log it (that would duplicate it in aggregators).
	if o.Type() == order.Market && !o.Price().IsZero() {
		return ErrMarketOrderPrice
	}
	if o.Type() == order.Limit && o.Size().IsZero() {
		return ErrOrderSizeIsZero
	}
	if o.Type() == order.Limit && o.Price().IsZero() {
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
// runs outside the lock to keep the critical section short. It uses WithoutCancel
// so a shutdown-canceled listener context cannot abort the read and overwrite the
// authoritative snapshot with an empty/stale result; a market adapter must bound
// the call with its own timeout.
func (b *Broker) refreshPositions(ctx context.Context) {
	positions := b.market.AccountPositions(context.WithoutCancel(ctx))
	b.mu.Lock()
	b.positions = positions
	b.mu.Unlock()
}

func (b *Broker) Listen(ctx context.Context, e any) {
	switch evt := e.(type) {
	case order.Order:
		if evt.Status() == order.Submitted {
			b.refreshPositions(ctx)
		}
	case indicator.Tick:
		// Remember the latest price per code so a market fill that carries no price
		// can still be valued (see applyOrderChange), advance the broker clock, and
		// sample equity once per day for the performance time series.
		b.mu.Lock()
		b.lastPrice[evt.Code] = evt.Price
		if evt.Date.After(b.now) {
			b.now = evt.Date
		}
		if !b.now.IsZero() {
			b.sampleEquityLocked(b.now)
		}
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
	//
	// WithoutCancel: this refresh is the authoritative half of completing a fill, so
	// a shutdown-canceled listener context must not abort it and leave b.positions
	// overwritten with an empty/stale snapshot while the order is still completed.
	// The adapter is responsible for bounding the call with its own timeout.
	positions := b.market.AccountPositions(context.WithoutCancel(ctx))

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
	// Only a booked fill moves the durable ledger (lots/realized/fees); other
	// order transitions touch state that is re-derived from the exchange on start.
	ledgerChanged := fillInc.GreaterThan(decimal.Zero)
	b.mu.Unlock()

	// Persist after releasing the lock so the (possibly slow) write never holds up
	// other broker operations. Fills are processed serially in the broker's single
	// listener goroutine, so writes stay ordered.
	if ledgerChanged {
		b.persist(ctx)
	}

	b.logger.Info("order changed", "id", o.ID(), "code", o.Item().Code, "action", evt.Action)
	b.notifyOrder(ctx, o.Copy())
}
