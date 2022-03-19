/*
 *  Copyright 2021 The Trader Authors
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
	"time"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/log"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
)

// Broker it is instead of human for buy , sell and etc
type Broker interface {
	Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.ExecType) error
	OrderCash(ctx context.Context, code string, howmuch int64, currentPrice float64, action order.Action, exec order.ExecType) error
	Cash() int64
	Position(code string) (position.Position, bool)
	SetCash(amount int64)
	SetCommission(percent float64)
}

type broker struct {
	cash             int64
	commission       float64
	orders           map[string]order.Order
	mu               sync.RWMutex
	eventEngine      event.Broadcaster
	positions        map[string]position.Position
	store            store.Store
	cashValueChanged bool
	log              log.Logger
	codeStateMachine map[string]bool
}

// NewBroker Init new broker with cash,commission
func NewBroker(log log.Logger, store store.Store, evt event.Broadcaster) Broker {
	bk := &broker{
		log:              log,
		store:            store,
		eventEngine:      evt,
		orders:           make(map[string]order.Order),
		positions:        map[string]position.Position{},
		codeStateMachine: map[string]bool{},
		cash:             400000,
	}
	return bk
}

func (b *broker) SetCash(amount int64) {
	b.cash = amount
}

func (b *broker) SetCommission(percent float64) {
	b.commission = percent
}

//OrderCash broker do buy/sell order from howmuch value and automatically calculate size
func (b *broker) OrderCash(ctx context.Context, code string, howmuch int64, currentPrice float64, action order.Action, exec order.ExecType) error {
	size := float64(howmuch) / currentPrice
	return b.Order(ctx, code, int64(size), currentPrice, action, exec)
}

func (b *broker) Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.ExecType) error {

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.codeStateMachine[code] {
		return PositionExists
	}

	b.codeStateMachine[code] = true
	o := order.NewOrder(code, action, exec, size, price, b.commission)

	switch o.Action() {
	case order.Buy:
		if int64(o.OrderPrice()+(o.OrderPrice()*(b.commission/100))) > b.cash {
			return NotEnoughCash
		}
	case order.Sell:
		if p, ok := b.positions[o.Code()]; !ok {
			return PositionNotExists
		} else {
			if p.Size > o.Size() {
				return LowSizeThenPosition
			}
		}
	}

	b.log.Debugf("order created: #@v", o)
	go b.submit(o)
	return nil
}

func (b *broker) submit(o order.Order) {
	o.Submit()
	b.notifyOrder(o.Copy())

	if err := b.store.Order(context.Background(), o); err != nil {
		o.Reject(err)
		b.notifyOrder(o.Copy())
		return
	}
	b.log.Debug("store order success")

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

	b.notifyCash(o.Copy())

	b.mu.Lock()
	b.codeStateMachine[o.Code()] = false
	b.mu.Unlock()
}

func (b *broker) appendPosition(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p, ok := b.positions[o.Code()]; ok {
		b.positions[o.Code()] = position.Position{
			Code:      o.Code(),
			Size:      p.Size + o.Size(),
			Price:     ((float64(p.Size) * p.Price) + o.OrderPrice()) / float64(p.Size+o.Size()),
			CreatedAt: time.Now(),
		}
		return
	}
	b.positions[o.Code()] = position.NewPosition(o)
}

func (b *broker) deletePosition(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.positions[o.Code()]; ok {
		if o.RemainPrice() == 0 {
			delete(b.positions, o.Code())
		}
	}
}

func (b *broker) notifyOrder(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.eventEngine.BroadCast(o)
}

func (b *broker) notifyCash(o order.Order) {
	//var value int64
	//switch o.Action() {
	//case order.Sell:
	//	value = int64(o.OrderPrice() - ((o.OrderPrice() * o.Commission()) / 100))
	//case order.Buy:
	//	value = -int64(o.OrderPrice() - ((o.OrderPrice() * o.Commission()) / 100))
	//}
	//fmt.Println(o.Commission())
	//fmt.Println("commision:", (o.OrderPrice()*o.Commission())/100)
	//fmt.Println(value)
	//fmt.Println(b.cash)
	//b.eventEngine.BroadCast(event.CashEvent{Before: b.cash, After: b.cash + value})
	//b.cash += value
}

func (b *broker) Cash() int64 {
	if b.cash == 0 {
		return b.store.Cash()
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	var value int64
	value = b.cash
	return value
}

func (b *broker) Position(code string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ps, ok := b.positions[code]
	return ps, ok
}

//TODO: test
func (b *broker) Listen(e interface{}) {

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
