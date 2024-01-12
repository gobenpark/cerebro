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
 */package broker

import (
	"context"
	"fmt"
	"sync"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"go.uber.org/zap"
)

type Broker struct {
	EventEngine      event.Broadcaster
	market           market.Market
	logger           log.Logger
	orders           []order.Order
	positions        []position.Position
	balance          int64
	mu               sync.RWMutex
	cashValueChanged bool
}

func NewDefaultBroker(eventEngine event.Broadcaster, store market.Market, logger log.Logger) *Broker {
	return &Broker{
		orders:           []order.Order{},
		EventEngine:      eventEngine,
		market:           store,
		cashValueChanged: false,
		logger:           logger,
	}
}

func (b *Broker) Order(ctx context.Context, o order.Order) error {
	if o.Type() == order.Market && o.Price() != 0 {
		b.logger.Error("invalid order price", "code", o.Code(), "price", o.Price(), "size", o.Size())
		return fmt.Errorf("invalid order price, market order price must be set 0")
	}

	if o.Type() == order.Limit && o.Size() == 0 {
		b.logger.Error("invalid order size", "code", o.Code(), "price", o.Price(), "size", o.Size())
		return ErrOrderSizeIsZero
	}

	if o.Type() == order.Limit && o.Price() == 0 {
		b.logger.Error("invalid order price", "code", o.Code(), "price", o.Price(), "size", o.Size())
		return ErrPriceIsZero
	}

	value := int64(o.OrderPrice() + (o.OrderPrice() * (b.market.Commission() / 100)))

	switch o.Action() {
	case order.Buy:
		if value > b.balance {
			return ErrNotEnoughCash
		}
	}
	go b.submit(ctx, o)
	return nil
}

// In goroutine
func (b *Broker) submit(ctx context.Context, o order.Order) {
	o.Submit()
	b.notifyOrder(o.Copy())
	start := b.balance

	if err := b.market.Order(ctx, o); err != nil {
		b.logger.Info("reject order", "order", o, "error", err)
		o.Reject()
		b.notifyOrder(o.Copy())
		return
	}

	if o.Action() == order.Sell {
		b.mu.Lock()
		b.balance += int64(o.OrderPrice() - (o.OrderPrice() * (b.market.Commission() / 100)))
		b.mu.Unlock()
	} else {
		b.mu.Lock()
		b.balance -= int64(o.OrderPrice() + (o.OrderPrice() * (b.market.Commission() / 100)))
		b.mu.Unlock()
	}
	zap.L().Debug("cash size change", zap.Int64("start", start), zap.Int64("end", b.balance))
}

func (b *Broker) notifyOrder(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.EventEngine.BroadCast(o)
}

func (b *Broker) Orders(code string) []order.Order {
	b.mu.RLock()
	defer b.mu.RUnlock()

	orders := []order.Order{}
	for i := range b.orders {
		if b.orders[i].Code() == code {
			orders = append(orders, b.orders[i])
		}
	}
	return orders
}

func (b *Broker) Position(ticker string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for i := range b.positions {
		if b.positions[i].Code == ticker {
			return b.positions[i], true
		}
	}
	return position.Position{}, false
}

func (b *Broker) completeOrder(o order.Order) {
	for i := range b.orders {
		if b.orders[i].ID() == o.ID() {
			b.orders = append(b.orders[:i], b.orders[i+1:]...)
		}
	}
}

func (b *Broker) Listen(e interface{}) {

	if m, ok := e.(market.MarketEvent); ok {
		switch evt := m.(type) {
		case market.ChangeOrderEvent:
			b.logger.Info("market change order", "message", m.(market.ChangeOrderEvent).Message, "id", m.(market.ChangeOrderEvent).ID, "action", m.(market.ChangeOrderEvent).Action)
			for i := range b.orders {
				if b.orders[i].ID() == m.(market.ChangeOrderEvent).ID {
					o := b.orders[i]
					switch evt.Action {
					case order.Accepted:
						o.Accept()
						b.logger.Info("order accepted", "id", o.ID(), "code", o.Code(), "price", o.Price(), "size", o.Size())
					case order.Completed:
						o.Complete()
						b.completeOrder(o)
						b.logger.Info("order completed", "id", o.ID(), "code", o.Code(), "price", o.Price(), "size", o.Size())
					case order.Canceled:
						o.Cancel()
						b.completeOrder(o)
						b.logger.Info("order canceled", "id", o.ID(), "code", o.Code(), "price", o.Price(), "size", o.Size())
					}
					b.notifyOrder(o)
				}
			}
		case market.ChangeBalanceEvent:
			b.logger.Info("market change balance", "message", m.(market.ChangeBalanceEvent).Message, "balance", m.(market.ChangeBalanceEvent).Balance)
			b.balance = m.(market.ChangeBalanceEvent).Balance
		}
	}
}
