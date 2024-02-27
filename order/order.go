/*
 *  Copyright 2021 The Cerebro Authors
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
package order

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/item"
	uuid "github.com/satori/go.uuid"
)

type (
	Action    int
	Status    int32
	OrderType int
)

const (
	Buy Action = iota + 1
	Sell
	Cancel
	Edit
)

const (
	None Status = iota
	Created
	Submitted
	Accepted
	Partial
	Completed
	Canceled
	Expired
	Margin
	Rejected
)

const (
	Market OrderType = iota + 1
	Close
	Limit
	Stop
	StopLimit
	StopTrail
	StopTrailLimit
	Historical
)

type Order interface {
	ID() string
	Item() *item.Item
	Type() OrderType
	Reject()
	Expire()
	Cancel()
	Margin()
	Submit()
	Accept()
	Partial(size int64)
	Complete()
	Status() Status
	Exec() OrderType
	OrderPrice() float64
	Action() Action
	Price() int64
	Size() int64
	RemainPrice() float64
	Copy() Order
	SetID(id string)
}

type order struct {
	createdAt     time.Time    `json:"createdAt"`
	updatedAt     time.Time    `json:"updatedAt"`
	item          *item.Item   `json:"item"`
	uuid          string       `json:"uuid"`
	action        Action       `json:"action"`
	OrderType     OrderType    `json:"orderType"`
	size          int64        `json:"size"`
	price         int64        `json:"price"`
	remainingSize int64        `json:"remainingSize"`
	mu            sync.RWMutex `json:"-"`
	status        Status       `json:"status"`
}

func NewOrder(item *item.Item, action Action, execType OrderType, size int64, price int64) Order {
	return &order{
		status:        Created,
		action:        action,
		OrderType:     execType,
		item:          item,
		uuid:          uuid.NewV4().String(),
		size:          size,
		price:         price,
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
		remainingSize: size,
	}
}

func (o *order) Type() OrderType {
	return o.OrderType
}

func (o *order) Accept() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Accepted
	o.updatedAt = time.Now()
}

func (o *order) SetID(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.uuid = id
}

func (o *order) Exec() OrderType {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.OrderType
}

func (o *order) Action() Action {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.action
}

func (o *order) Item() *item.Item {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.item
}

func (o *order) ID() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.uuid
}

func (o *order) Reject() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Rejected
	o.updatedAt = time.Now()
}

func (o *order) Expire() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Expired
	o.updatedAt = time.Now()
}

func (o *order) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Canceled
	o.updatedAt = time.Now()
}

func (o *order) Margin() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Margin
	o.updatedAt = time.Now()
}

func (o *order) Partial(size int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.remainingSize -= size
	o.status = Partial
	o.updatedAt = time.Now()
}

func (o *order) Submit() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Submitted
	o.updatedAt = time.Now()
}

func (o *order) Complete() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.remainingSize = 0
	o.status = Completed
}

func (o *order) Status() Status {
	var value Status
	o.mu.RLock()
	defer o.mu.RUnlock()
	value = o.status
	return value
}

func (o *order) OrderPrice() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return float64(o.price) * float64(o.size)
}

func (o *order) RemainPrice() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return float64(o.price) * float64(o.remainingSize)
}

func (o *order) Price() int64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.price
}

func (o *order) Size() int64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.size
}

func (o *order) Copy() Order {
	o.mu.Lock()
	defer o.mu.Unlock()
	return &order{
		status:        o.status,
		action:        o.action,
		OrderType:     o.OrderType,
		item:          o.item,
		uuid:          o.uuid,
		size:          o.size,
		price:         o.price,
		createdAt:     o.createdAt,
		updatedAt:     o.updatedAt,
		remainingSize: o.remainingSize,
	}
}

func (o *order) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"item":      o.item,
		"price":     o.price,
		"size":      o.size,
		"createdAt": o.createdAt.Format("2006-01-02 15:04:05"),
	}
	return json.Marshal(data)
}
