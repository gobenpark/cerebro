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
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/satori/go.uuid"
)

// Broker it is instead of human for buy , sell and etc
type Broker interface {
	Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.ExecType) error
	GetCash() int64
	GetPosition() []position.Position
	SetStore(store store.Store)
}

type broker struct {
	Cash        int64
	Commission  float64
	orders      map[string]*order.Order
	mu          sync.Mutex
	eventEngine event.Broadcaster
	positions   map[string][]position.Position
	store       store.Store
}

// NewBroker Init new broker with cash,commission
func NewBroker(store store.Store, evt event.Broadcaster) Broker {
	return &broker{store: store, eventEngine: evt}
}

func (b *broker) Order(ctx context.Context, code string, size int64, price float64, action order.Action, exec order.ExecType) error {
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

	if err := b.store.Order(ctx, &o); err != nil {
		return err
	}

	b.eventEngine.BroadCast(o)

	return nil
}

func (b *broker) GetCash() int64 {
	return b.store.Cash()
}

func (b *broker) GetPosition() []position.Position {
	return b.store.Positions()
}

func (b *broker) SetStore(store store.Store) {
	b.store = store
}

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
