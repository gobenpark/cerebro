/*
 *  Copyright 2023 The Trader Authors
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
	"go.uber.org/zap"
)

// Broker it is instead of human for buy , sell and etc
//type Broker interface {
//	Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.OrderType) error
//	OrderCash(ctx context.Context, code string, amount float64, currentPrice float64, action order.Action, exec order.OrderType) error
//	Cash() int64
//	Position(code string) (position.Position, bool)
//	SetCash(amount int64)
//	SetCommission(percent float64)
//}

type Broker struct {
	orders           map[string]order.Order
	mu               sync.RWMutex
	EventEngine      event.Broadcaster
	positions        map[string]position.Position
	store            store.Store
	cashValueChanged bool
	log              log.Logger
	codeStateMachine map[string]bool
	Commission       float64
	Cash             int64
}

func NewBroker(eventEngine event.Broadcaster, store store.Store, commission float64, cash int64) *Broker {
	return &Broker{
		orders:           map[string]order.Order{},
		EventEngine:      eventEngine,
		positions:        map[string]position.Position{},
		store:            store,
		cashValueChanged: false,
		log:              nil,
		codeStateMachine: map[string]bool{},
		Commission:       commission,
		Cash:             cash,
	}
}

func (b *Broker) SetCommission(percent float64) {
	b.Commission = percent
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
	if b.codeStateMachine[code] {
		return PositionExists
	}

	b.codeStateMachine[code] = true

	o := order.NewOrder(code, action, ot, size, price, b.Commission)

	value := int64(o.OrderPrice() + (o.OrderPrice() * (b.Commission / 100)))
	// validation check
	switch o.Action() {
	case order.Buy:
		if value > b.Cash {
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

	go b.submit(ctx, o)
	return nil
}

// In goroutine
func (b *Broker) submit(ctx context.Context, o order.Order) {
	o.Submit()
	b.notifyOrder(o.Copy())
	start := b.Cash

	if err := b.store.Order(ctx, o); err != nil {
		o.Reject(err)
		b.notifyOrder(o.Copy())
		return
	}

	zap.L().Info("store order success")
	o.Complete()
	b.notifyOrder(o.Copy())

	if o.Action() == order.Sell {
		b.mu.Lock()
		b.Cash += int64(o.OrderPrice() - (o.OrderPrice() * (b.Commission / 100)))
		b.mu.Unlock()
		b.deletePosition(o)
	} else {
		b.mu.Lock()
		b.Cash -= int64(o.OrderPrice() + (o.OrderPrice() * (b.Commission / 100)))
		b.mu.Unlock()
		b.appendPosition(o)
	}
	zap.L().Debug("cash size change", zap.Int64("start", start), zap.Int64("end", b.Cash))
	b.notifyCash(o.Copy())

	b.mu.Lock()
	b.codeStateMachine[o.Code()] = false
	b.mu.Unlock()
}

func (b *Broker) appendPosition(o order.Order) {
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

func (b *Broker) deletePosition(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.positions[o.Code()]; ok {
		if o.RemainPrice() == 0 {
			delete(b.positions, o.Code())
		}
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

func (b *Broker) Position(code string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ps, ok := b.positions[code]
	return ps, ok
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
