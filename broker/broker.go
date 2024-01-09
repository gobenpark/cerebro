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
	positions        map[string]position.Position
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
	defer b.mu.Unlock()
	if o.Type() == order.Market && o.Price() != 0 {
		return fmt.Errorf("invalid order price, market order price must be set 0")
	}

	if o.Size() == 0 {
		return ErrOrderSizeIsZero
	}

	if o.Price() == 0 {
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
	b.notifyCash(o.Copy())
}

func (b *Broker) notifyOrder(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.EventEngine.BroadCast(o)
}

func (b *Broker) notifyCash(o order.Order) {
	//var value int64
	//switch o.Action() {
	//case order.Sell:
	//	value = int64(o.OrderPrice() - ((o.OrderPrice() * o.commission()) / 100))
	//case order.Buy:
	//	value = -int64(o.OrderPrice() - ((o.OrderPrice() * o.commission()) / 100))
	//}
	//fmt.Println(o.commission())
	//fmt.Println("commision:", (o.OrderPrice()*o.commission())/100)
	//fmt.Println(value)
	//fmt.Println(b.cash)
	//b.eventEngine.BroadCast(event.CashEvent{Before: b.cash, After: b.cash + value})
	//b.cash += value
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

func (b *Broker) Position(code string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if p, ok := b.positions[code]; ok {
		return p, true
	}
	return position.Position{}, false
}

func (b *Broker) Cancel(o order.Order) error {
	return nil
}

func (b *Broker) Listen(e interface{}) {
	if o, ok := e.(order.Order); ok {
		switch o.Status() {
		case order.Rejected, order.Accepted, order.Canceled, order.Expired:
		}

		//b.positions = b.market.Positions()
	}
}
