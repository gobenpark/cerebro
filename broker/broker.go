//go:generate mockgen -source=./broker.go -destination=./mock/mock_broker.go
package broker

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/satori/go.uuid"
)

type DefaultBroker struct {
	sync.RWMutex
	cash       int64
	commission float32
	orders     map[string]*order.Order
	mu         sync.Mutex
	event      chan<- event.Event
	positions  map[string]position.Position
}

//NewBroker Init new broker with cash,commission
func NewBroker(cash int64, commission float32) *DefaultBroker {

	return &DefaultBroker{
		cash:       cash,
		commission: commission,
		orders:     make(map[string]*order.Order),
		positions:  make(map[string]position.Position),
	}
}

func (b *DefaultBroker) Buy(code string, size int64, price float64) string {
	uid := uuid.NewV4().String()
	o := &order.Order{
		Code:      code,
		UUID:      uid,
		Status:    order.Submitted,
		OrderType: order.Buy,
		Size:      size,
		Price:     price,
		Broker:    b,
	}
	b.orders[o.UUID] = o
	b.transmit(o)
	b.event <- event.Event{UUID: uuid.NewV4().String()}
	return uid
}

func (b *DefaultBroker) Sell(code string, size int64, price float64) string {
	uid := uuid.NewV4().String()
	o := &order.Order{
		Code:      code,
		UUID:      uid,
		OrderType: order.Sell,
		Size:      size,
		Price:     price,
		Broker:    b,
	}
	b.orders[o.UUID] = o
	b.transmit(o)

	return uid
}
func (b *DefaultBroker) SetEventCh(ch chan<- event.Event) {
	b.event = ch
}

func (b *DefaultBroker) Cancel(uid string) {
	if o, ok := b.orders[uid]; ok {
		o.Cancel()
		//TODO: fix cancel event send
		b.event <- event.Event{UUID: uuid.NewV4().String()}
		return
	}
	b.event <- event.Event{UUID: uuid.NewV4().String()}
}

func (b *DefaultBroker) Submit(uid string) {
	if ord, ok := b.orders[uid]; ok {
		ord.Submit()
		b.event <- event.Event{UUID: uuid.NewV4().String()}
		return
	}
}

func (b *DefaultBroker) GetPosition(code string) (position.Position, error) {
	if p, ok := b.positions[code]; ok {
		return p, nil
	}
	return position.Position{}, fmt.Errorf("not exist code %s", code)
}

func (b *DefaultBroker) SetCash(cash int64) {
	atomic.StoreInt64(&b.cash, cash)
}

func (b *DefaultBroker) transmit(o *order.Order) {
	//TODO: Order create
}

func (b *DefaultBroker) AddOrderHistory() {
	panic("implement me")
}

func (b *DefaultBroker) SetFundHistory() {
	panic("implement me")
}

func (b *DefaultBroker) CommissionInfo() {
	panic("implement me")
}
