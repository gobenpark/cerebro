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
	"sync"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"go.uber.org/zap"
)

type Broker interface {
	//OrderCash(ctx context.Context, code string, amount float64, currentPrice int64, action order.Action, exec order.OrderType) error
	SubmitOrder(ctx context.Context, code string, size int64, price int64, action order.Action, ot order.OrderType) error
	Orders(code string) []order.Order
	Position(code string) (position.Position, bool)
	Cash() int64
	Cancel(o order.Order) error
	event.Listener
}

type DefaultBroker struct {
	EventEngine      event.Broadcaster
	market           market.Market
	logger           log.Logger
	orders           []order.Order
	positions        map[string]position.Position
	commission       float64
	cash             int64
	mu               sync.RWMutex
	cashValueChanged bool
}

func NewDefaultBroker(eventEngine event.Broadcaster, store market.Market, logger log.Logger) *DefaultBroker {
	return &DefaultBroker{
		orders:           []order.Order{},
		EventEngine:      eventEngine,
		market:           store,
		cashValueChanged: false,
		logger:           logger,
	}
}

func (b *DefaultBroker) Setcommission(percent float64) {
	b.commission = percent
}

// OrderCash broker do buy/sell order from how much value and automatically calculate size
func (b *DefaultBroker) OrderCash(ctx context.Context, code string, amount float64, currentPrice int64, action order.Action, exec order.OrderType) error {

	size := amount / float64(currentPrice)
	return b.SubmitOrder(ctx, code, int64(size), currentPrice, action, exec)
}

func (b *DefaultBroker) SubmitOrder(ctx context.Context, code string, size int64, price int64, action order.Action, ot order.OrderType) error {

	b.mu.Lock()
	defer b.mu.Unlock()
	if ot == order.Market {
		price = 0
	}

	if size == 0 {
		return ErrOrderSizeIsZero
	}

	if price == 0 && ot != order.Market {
		return ErrPriceIsZero
	}

	o := order.NewOrder(code, action, ot, size, price, b.commission)

	value := int64(o.OrderPrice() + (o.OrderPrice() * (b.commission / 100)))
	// validation check
	switch o.Action() {
	case order.Buy:
		if value > b.cash {
			return ErrNotEnoughCash
		}
	}

	b.orders = append(b.orders, o)
	go b.submit(ctx, o)
	return nil
}

// In goroutine
func (b *DefaultBroker) submit(ctx context.Context, o order.Order) {
	o.Submit()
	b.notifyOrder(o.Copy())
	start := b.cash

	//if err := b.market.Order(ctx, o); err != nil {
	//	b.logger.Info("reject order", "order", o, "error", err)
	//	o.Reject()
	//	b.notifyOrder(o.Copy())
	//	return
	//}

	if o.Action() == order.Sell {
		b.mu.Lock()
		b.cash += int64(o.OrderPrice() - (o.OrderPrice() * (b.commission / 100)))
		b.mu.Unlock()
	} else {
		b.mu.Lock()
		b.cash -= int64(o.OrderPrice() + (o.OrderPrice() * (b.commission / 100)))
		b.mu.Unlock()
	}
	zap.L().Debug("cash size change", zap.Int64("start", start), zap.Int64("end", b.cash))
	b.notifyCash(o.Copy())
}

func (b *DefaultBroker) notifyOrder(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.EventEngine.BroadCast(o)
}

func (b *DefaultBroker) notifyCash(o order.Order) {
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

func (b *DefaultBroker) Cash() int64 {
	return b.cash
}

func (b *DefaultBroker) Orders(code string) []order.Order {
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

func (b *DefaultBroker) Position(code string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if p, ok := b.positions[code]; ok {
		return p, true
	}
	return position.Position{}, false
}

func (b *DefaultBroker) Cancel(o order.Order) error {
	return nil
}

func (b *DefaultBroker) Listen(e interface{}) {
	if o, ok := e.(order.Order); ok {
		switch o.Status() {
		case order.Rejected, order.Accepted, order.Canceled, order.Expired:
		}

		//b.positions = b.market.Positions()
	}
}
