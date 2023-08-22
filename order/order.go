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
	"sync"
	"time"

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
)

const (
	Created Status = iota + 1
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
	Code() string
	Reject(err error)
	Expire()
	Cancel()
	Margin()
	Submit()
	Partial(size int64)
	Complete()
	Status() Status
	Exec() OrderType
	OrderPrice() float64
	Action() Action
	Price() float64
	Size() int64
	RemainPrice() float64
	Copy() Order
	Commission() float64
}

type order struct {
	status        Status `json:"status,omitempty"`
	action        Action `json:"action,omitempty"`
	OrderType     `json:"exec_type,omitempty"`
	commission    float64      `json:"commission,omitempty"`
	code          string       `json:"code" form:"code" json:"code,omitempty"`
	uuid          string       `json:"uuid" form:"uuid" json:"uuid,omitempty"`
	size          int64        `json:"size" form:"size" json:"size,omitempty"`
	price         float64      `json:"price" form:"price" json:"price,omitempty"`
	createdAt     time.Time    `json:"createdAt" form:"created_at" json:"created_at"`
	updatedAt     time.Time    `json:"updatedAt" form:"updated_at" json:"updated_at"`
	mu            sync.RWMutex `json:"-"`
	remainingSize int64        `json:"remaining_size,omitempty"`
}

func NewOrder(code string, action Action, execType OrderType, size int64, price float64, commission float64) Order {
	return &order{
		status:        Created,
		action:        action,
		OrderType:     execType,
		code:          code,
		uuid:          uuid.NewV4().String(),
		size:          size,
		price:         price,
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
		remainingSize: size,
		commission:    commission,
	}
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

func (o *order) Code() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.code
}

func (o *order) ID() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.uuid
}

func (o *order) Reject(err error) {
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
	return o.price * float64(o.size)
}

func (o *order) Commission() float64 {
	var com float64
	o.mu.Lock()
	com = o.commission
	o.mu.Unlock()
	return com

}

func (o *order) RemainPrice() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.price * float64(o.remainingSize)
}

func (o *order) Price() float64 {
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
		code:          o.code,
		uuid:          o.uuid,
		size:          o.size,
		price:         o.price,
		createdAt:     o.createdAt,
		updatedAt:     o.updatedAt,
		remainingSize: o.remainingSize,
		commission:    o.commission,
	}
}
