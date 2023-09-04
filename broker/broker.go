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
	"sync"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/store"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// Broker it's instead of human for buy, sell and etc
//type Broker interface {
//	Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.OrderType) error
//	OrderCash(ctx context.Context, code string, amount float64, currentPrice float64, action order.Action, exec order.OrderType) error
//	Cash() int64
//	Position(code string) (position.Position, bool)
//	SetCash(amount int64)
//	Setcommission(percent float64)
//}

type Broker struct {
	orders           []order.Order
	mu               sync.RWMutex
	EventEngine      event.Broadcaster
	positions        []position.Position
	store            store.Store
	cashValueChanged bool
	logger           log.Logger
	orderState       map[string]bool
	commission       float64
	cash             int64
}

func NewBroker(eventEngine event.Broadcaster, store store.Store, commission float64, cash int64, logger log.Logger) *Broker {
	return &Broker{
		orders:           []order.Order{},
		EventEngine:      eventEngine,
		positions:        []position.Position{},
		store:            store,
		cashValueChanged: false,
		logger:           logger,
		orderState:       map[string]bool{},
		commission:       commission,
		cash:             cash,
	}
}

func (b *Broker) Setcommission(percent float64) {
	b.commission = percent
}

// TODO: impelement
// OrderCash broker do buy/sell order from how much value and automatically calculate size
func (b *Broker) OrderCash(ctx context.Context, code string, amount float64, currentPrice float64, action order.Action, exec order.OrderType) error {

	size := amount / currentPrice
	return b.Order(ctx, code, int64(size), currentPrice, action, exec)
}

func (b *Broker) Order(ctx context.Context, code string, size int64, price float64, action order.Action, ot order.OrderType) error {

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.orderState[code] {
		return ErrPositionExists
	}

	b.orderState[code] = true

	o := order.NewOrder(code, action, ot, size, price, b.commission)

	value := int64(o.OrderPrice() + (o.OrderPrice() * (b.commission / 100)))
	// validation check
	switch o.Action() {
	case order.Buy:
		if value > b.cash {
			return ErrNotEnoughCash
		}
	case order.Sell:

		order, ok := lo.Find(b.positions, func(item position.Position) bool {
			return item.Code == o.Code()
		})
		if !ok {
			return ErrPositionNotExists
		}

		if order.Size > o.Size() {
			return ErrLowSizeThenPosition
		}
	}

	go b.submit(ctx, o)
	return nil
}

// In goroutine
func (b *Broker) submit(ctx context.Context, o order.Order) {
	o.Submit()
	b.notifyOrder(o.Copy())
	start := b.cash

	if err := b.store.Order(ctx, o); err != nil {
		o.Reject(err)
		b.notifyOrder(o.Copy())
		return
	}

	o.Complete()
	b.notifyOrder(o.Copy())

	if o.Action() == order.Sell {
		b.mu.Lock()
		b.cash += int64(o.OrderPrice() - (o.OrderPrice() * (b.commission / 100)))
		b.mu.Unlock()
		b.deletePosition(o)
	} else {
		b.mu.Lock()
		b.cash -= int64(o.OrderPrice() + (o.OrderPrice() * (b.commission / 100)))
		b.mu.Unlock()
		b.appendPosition(o)
	}
	zap.L().Debug("cash size change", zap.Int64("start", start), zap.Int64("end", b.cash))
	b.notifyCash(o.Copy())

	b.mu.Lock()
	b.orderState[o.Code()] = false
	b.mu.Unlock()
}

func (b *Broker) appendPosition(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()

	preOrder, index, ok := lo.FindIndexOf(b.positions, func(item position.Position) bool {
		return item.Code == o.Code()
	})
	if ok {
		b.positions[index] = position.Position{
			Code:  o.Code(),
			Size:  preOrder.Size + o.Size(),
			Price: ((float64(preOrder.Size) * preOrder.Price) + o.OrderPrice()) / float64(preOrder.Size+o.Size()),
		}
	}
	b.positions = append(b.positions, position.NewPosition(o))
}

func (b *Broker) deletePosition(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, index, ok := lo.FindIndexOf(b.positions, func(item position.Position) bool {
		return item.Code == o.Code()
	})
	if ok {
		b.positions = lo.Drop(b.positions, index)
	}
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

func (b *Broker) Cash() int64 {
	return b.cash
}

func (b *Broker) Position(code string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return lo.Find(b.positions, func(item position.Position) bool {
		return item.Code == code
	})
}

// TODO: test
func (b *Broker) Listen(e interface{}) {

	//if evt, ok := e.(event.OrderEvent); ok {
	//	switch evt.State {
	//	case "cancel":
	//	case "done":
	//	case "wait":
	//		fmt.Println(b.positions)
	//		fmt.Println("wait")
	//	}
	//}
}
