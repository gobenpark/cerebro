package broker

import (
	"sync"
	"sync/atomic"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	order2 "github.com/gobenpark/trader/order"
	"github.com/satori/go.uuid"
)

//go:generate mockgen -source=./broker.go -destination=./mock/mock_broker.go

type DefaultBroker struct {
	cash       int64
	commission float32
	orders     map[string]*order2.Order
	mu         sync.Mutex
	event      chan<- event.Event
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

//NewBroker Init new broker with cash,commission
func NewBroker(cash int64, commission float32) domain.Broker {
	return &DefaultBroker{
		cash:       cash,
		commission: commission,
	}
}

func (b *DefaultBroker) Buy(size int64, price float64) {
	uid := uuid.NewV4().String()
	order := &order2.Order{
		UUID:      uid,
		Status:    order2.Submitted,
		OrderType: order2.Buy,
		Size:      size,
		Price:     price,
		Broker:    b,
	}
	b.orders[order.UUID] = order
	b.Submit(order.UUID)
}

func (b *DefaultBroker) Sell(size int64, price float64) {
	uid := uuid.NewV4().String()
	order := &order2.Order{
		UUID:      uid,
		OrderType: order2.Sell,
		Size:      size,
		Price:     price,
		Broker:    b,
	}
	b.orders[order.UUID] = order
	b.Submit(order.UUID)
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

func (b *DefaultBroker) GetPosition() {
	panic("implement me")
}

func (b *DefaultBroker) SetCash(cash int64) {
	atomic.StoreInt64(&b.cash, cash)
}
