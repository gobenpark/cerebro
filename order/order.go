package order

import (
	"time"
)

type (
	OType    int
	Status   int
	ExecType int
)

const (
	Buy OType = iota + 1
	Sell

	Created Status = iota + 1
	Submitted
	Excuting
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
	Status
	OType
	Code       string    `json:"code"`
	UUID       string    `json:"uuid"`
	Size       int64     `json:"size"`
	Price      float64   `json:"price"`
	CreatedAt  time.Time `json:"createdAt"`
	ExecutedAt time.Time `json:"executedAt"`
}

func (o *Order) Reject(err error) {
	o.Status = Rejected
	o.ExecutedAt = time.Now()
}

func (o *Order) Expire() {
	o.Status = Expired
	o.ExecutedAt = time.Now()
}

func (o *Order) Cancel() {
	o.Status = Canceled
	o.ExecutedAt = time.Now()
}

func (o *Order) Margin() {
	o.Status = Margin
	o.ExecutedAt = time.Now()
}

func (o *Order) Partial() {
	o.Status = Partial
	o.ExecutedAt = time.Now()
}

func (o *Order) Execute() {
	o.Status = Excuting
	o.ExecutedAt = time.Now()
}

func (o *Order) Submit() {
	o.Status = Submitted
	o.CreatedAt = time.Now()
}
