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
	"sync"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/event"
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
	// mu guards orders, positions, and balance.
	mu sync.RWMutex
	// wg tracks in-flight submit goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
	// risk is the optional pre-trade gate consulted in Order; nil means no gate.
	risk *risk.Manager
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
		Positions: slices.Clone(b.positions),
		Open:      slices.Clone(b.orders),
	}
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

var _ Submitter = (*Broker)(nil)

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
	switch evt.Action {
	case order.Accepted:
		o.Accept()
	case order.Completed:
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
		o.Partial(evt.FilledSize)
	default:
		// None/Created/Submitted are not delivered as exchange events; nothing to apply.
	}
	// Terminal statuses end the order's lifecycle: drop it from the open set so
	// its reserved cash is released. Non-terminal updates (e.g. Accepted) keep
	// the order open and its reservation in place.
	if isTerminalStatus(evt.Action) {
		b.orders = slices.Delete(b.orders, idx, idx+1)
	}
	b.positions = positions
	b.mu.Unlock()

	b.logger.Info("order changed", "id", o.ID(), "code", o.Item().Code, "action", evt.Action)
	b.notifyOrder(ctx, o.Copy())
}
