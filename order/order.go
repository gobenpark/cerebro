package order

import (
	"time"

	"github.com/gobenpark/trader/broker"
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
	UUID string
	Status
	Broker     broker.Broker
	OrderType  OType
	Size       int64
	Price      float64
	CreatedAt  time.Time
	ExecutedAt time.Time
}

func (o *Order) Reject() {
}

func (*Order) Expire() {

}

func (o *Order) Cancel() {
	o.Status = Canceled
	o.ExecutedAt = time.Now()
}

func (*Order) Margin() {

}

func (*Order) Partial() {

}

func (*Order) Execute() {

}

func (o *Order) Submit() {
	o.Status = Submitted
	o.CreatedAt = time.Now()
}
