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
	"fmt"
	"sync"
	"time"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/satori/go.uuid"
)

type Broker interface {
	Order(code string, size int64, price float64, action order.Action, exec order.ExecType) error
	GetCash() int64
	GetPosition()
}

type broker struct {
	Cash        int64
	Commission  float64
	orders      map[string]*order.Order
	mu          sync.Mutex
	eventEngine event.Broadcaster
	positions   map[string][]position.Position
	Store       store.Store
}

// NewBroker Init new broker with cash,commission
func NewBroker() Broker {
	return &broker{}
}

func (b *broker) Order(code string, size int64, price float64, action order.Action, exec order.ExecType) error {
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

	_ = o

	return nil
}

func (b *broker) GetCash() int64 {
	panic("implement me")
}

func (b *broker) GetPosition() {
	panic("implement me")
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
