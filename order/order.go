/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
 */
package order

import (
	"sync"
	"time"
)

type (
	OType    int
	Status   int32
	ExecType int
)

const (
	Buy OType = iota + 1
	Sell

	Created Status = iota + 1
	Submitted
	Accepted
	Partial
	Completed
	Canceled
	Expired
	Margin
	Rejected

	Market ExecType = iota + 1
	Close
	Limit
	Stop
	StopLimit
	StopTrail
	StopTrailLimit
	Historical
)

type Order struct {
	status Status
	OType
	ExecType
	Code       string    `json:"code"`
	UUID       string    `json:"uuid"`
	Size       int64     `json:"size"`
	Price      float64   `json:"price"`
	CreatedAt  time.Time `json:"createdAt"`
	ExecutedAt time.Time `json:"executedAt"`
	mu         sync.RWMutex
	StoreUID   string `json:"-"`
}

func (o *Order) Reject(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Rejected
	o.ExecutedAt = time.Now()
}

func (o *Order) Expire() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Expired
	o.ExecutedAt = time.Now()
}

func (o *Order) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Canceled
	o.ExecutedAt = time.Now()
}

func (o *Order) Margin() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Margin
	o.ExecutedAt = time.Now()
}

func (o *Order) Partial() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Partial
	o.ExecutedAt = time.Now()
}

func (o *Order) Submit() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Submitted
	o.CreatedAt = time.Now()
	o.ExecutedAt = time.Now()
}

func (o *Order) Complete() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = Completed
}

func (o *Order) Status() Status {
	var value Status
	o.mu.RLock()
	defer o.mu.RUnlock()
	value = o.status
	return value
}
