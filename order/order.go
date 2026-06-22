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

	uuid "github.com/satori/go.uuid"
	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/item"
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
	Partial(size decimal.Decimal)
	Complete()
	Status() Status
	Exec() OrderType
	OrderPrice() decimal.Decimal
	Action() Action
	Price() decimal.Decimal
	Size() decimal.Decimal
	RemainPrice() decimal.Decimal
	Copy() Order
	SetID(id string)
}

type order struct {
	createdAt     time.Time
	updatedAt     time.Time
	item          *item.Item
	uuid          string
	action        Action
	OrderType     OrderType `json:"orderType"`
	size          decimal.Decimal
	price         decimal.Decimal
	remainingSize decimal.Decimal
	mu            sync.RWMutex
	status        Status
}

func NewOrder(item *item.Item, action Action, execType OrderType, size, price decimal.Decimal) Order {
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
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.OrderType
}

func (o *order) Action() Action {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.action
}

func (o *order) Item() *item.Item {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.item
}

func (o *order) ID() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
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

func (o *order) Partial(size decimal.Decimal) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.remainingSize = o.remainingSize.Sub(size)
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
	o.remainingSize = decimal.Zero
	o.status = Completed
	o.updatedAt = time.Now()
}

func (o *order) Status() Status {
	var value Status
	o.mu.RLock()
	defer o.mu.RUnlock()
	value = o.status
	return value
}

func (o *order) OrderPrice() decimal.Decimal {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.price.Mul(o.size)
}

func (o *order) RemainPrice() decimal.Decimal {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.price.Mul(o.remainingSize)
}

func (o *order) Price() decimal.Decimal {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.price
}

func (o *order) Size() decimal.Decimal {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.size
}

func (o *order) Copy() Order {
	o.mu.RLock()
	defer o.mu.RUnlock()
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
	data := map[string]any{
		"item":      o.item,
		"price":     o.price,
		"size":      o.size,
		"createdAt": o.createdAt.Format("2006-01-02 15:04:05"),
	}
	return json.Marshal(data)
}
