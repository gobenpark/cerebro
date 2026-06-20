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

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
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
	balance int64
	// mu guards orders, positions, and balance.
	mu sync.RWMutex
	// wg tracks in-flight submit goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
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

// orderValue returns the cash an order commits, including commission.
func (b *Broker) orderValue(o order.Order) int64 {
	return int64(o.OrderPrice() + (o.OrderPrice() * (b.market.Commission() / 100)))
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
func (b *Broker) reservedLocked() int64 {
	var reserved int64
	for i := range b.orders {
		if b.orders[i].Action() == order.Buy {
			reserved += b.orderValue(b.orders[i])
		}
	}
	return reserved
}

// Balance returns the settled cash reported by the exchange.
func (b *Broker) Balance() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.balance
}

// Available returns the cash available for new buy orders:
// settled balance minus cash reserved by open buy orders.
func (b *Broker) Available() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.balance - b.reservedLocked()
}

func (b *Broker) Order(ctx context.Context, o order.Order, safe bool) error {
	if o.Type() == order.Market && o.Price() != 0 {
		b.logger.Error("invalid order price", "code", o.Item().Code, "price", o.Price(), "size", o.Size())
		return fmt.Errorf("invalid order price, market order price must be set 0")
	}

	if o.Type() == order.Limit && o.Size() == 0 {
		b.logger.Error("invalid order size", "code", o.Item().Code, "price", o.Price(), "size", o.Size())
		return ErrOrderSizeIsZero
	}

	if o.Type() == order.Limit && o.Price() == 0 {
		b.logger.Error("invalid order price", "code", o.Item().Code, "price", o.Price(), "size", o.Size())
		return ErrPriceIsZero
	}

	b.mu.Lock()
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
		if value := b.orderValue(o); value > b.balance-b.reservedLocked() {
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
	default:
		// None/Created/Submitted/Partial are not delivered as terminal or
		// accepted exchange events; nothing to apply.
	}
	// Terminal statuses end the order's lifecycle: drop it from the open set so
	// its reserved cash is released. Non-terminal updates (e.g. Accepted) keep
	// the order open and its reservation in place.
	if isTerminalStatus(evt.Action) {
		b.orders = slices.Delete(b.orders, idx, idx+1)
	}
	b.mu.Unlock()

	b.logger.Info("order changed", "id", o.ID(), "code", o.Item().Code, "action", evt.Action)
	b.notifyOrder(ctx, o.Copy())
	b.refreshPositions()
}
