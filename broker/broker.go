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
	"fmt"
	"sync"
	"time"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/log"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/satori/go.uuid"
)

// Broker it is instead of human for buy , sell and etc
type Broker interface {
	Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.ExecType)
	Cash() int64
	Positions() map[string]position.Position
	SetCash(amount int64)
}

type broker struct {
	cash             int64
	Commission       float64
	orders           map[string]order.Order
	mu               sync.RWMutex
	eventEngine      event.Broadcaster
	positions        map[string]position.Position
	store            store.Store
	cashValueChanged bool
	log              log.Logger
}

// NewBroker Init new broker with cash,commission
func NewBroker(log log.Logger, store store.Store, evt event.Broadcaster) Broker {
	bk := &broker{log: log, store: store, eventEngine: evt, orders: make(map[string]order.Order)}
	return bk
}

func (b *broker) SetCash(amount int64) {
	b.cash = amount
}

func (b *broker) Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.ExecType) {
	uid := uuid.NewV4().String()

	o := order.Order{
		Action:    action,
		ExecType:  exec,
		Code:      code,
		UUID:      uid,
		Size:      size,
		Price:     price,
		CreatedAt: time.Now(),
	}
	b.log.Debugf("order created: #@v", o)

	go b.submit(&o)
	b.notifyOrder(&o)
}

func (b *broker) submit(o *order.Order) {
	o.Submit()
	//TODO: context

	if err := b.store.Order(context.Background(), o); err != nil {
		o.Reject(err)
		b.notifyOrder(o)
		return
	}
	b.log.Debug("store order success")

	o.Complete()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.orders[o.UUID] = *o
	b.positions = b.store.Positions()
	b.cashValueChanged = true

	b.notifyCash()
}

func (b *broker) notifyOrder(o *order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.eventEngine.BroadCast(o)
}

func (b *broker) notifyCash() {
	changedCash := b.store.Cash()
	b.eventEngine.BroadCast(event.CashEvent{Before: b.cash, After: changedCash})
	b.cash = changedCash
}

func (b *broker) Cash() int64 {
	if b.cash == 0 {
		return b.store.Cash()
	}
	return b.cash
}

func (b *broker) Positions() map[string]position.Position {
	b.mu.RLock()
	defer b.mu.RUnlock()

	tmap := map[string]position.Position{}
	if b.positions == nil {
		b.positions = b.store.Positions()
	}

	for k, v := range b.positions {
		tmap[k] = v
	}
	return tmap
}

//TODO: test
func (b *broker) Listen(e interface{}) {

	if evt, ok := e.(event.OrderEvent); ok {
		switch evt.State {
		case "cancel":
		case "done":
		case "wait":
			fmt.Println(b.positions)
			fmt.Println("wait")
		}
	}
}
