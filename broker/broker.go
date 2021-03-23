//go:generate mockgen -source=./broker.go -destination=./mock/mock_broker.go
package broker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/satori/go.uuid"
)

type DefaultBroker struct {
	sync.RWMutex
	cash        int64
	commission  float32
	orders      map[string]*order.Order
	mu          sync.Mutex
	eventEngine event.EventBroadcaster
	positions   map[string]position.Position
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
		Status:    order.Submitted,
		OType:     order.Buy,
		Code:      code,
		UUID:      uid,
		Size:      size,
		Price:     price,
		CreatedAt: time.Now(),
	}
	b.orders[o.UUID] = o
	b.transmit(o)
	return uid
}

func (b *DefaultBroker) Sell(code string, size int64, price float64) string {
	uid := uuid.NewV4().String()
	o := &order.Order{
		Code:  code,
		UUID:  uid,
		OType: order.Sell,
		Size:  size,
		Price: price,
	}
	b.orders[o.UUID] = o
	b.transmit(o)
	return uid
}

func (b *DefaultBroker) Cancel(uid string) {
	if o, ok := b.orders[uid]; ok {
		o.Cancel()
		b.eventEngine.BroadCast(o)
		return
	}
}

func (b *DefaultBroker) Submit(uid string) {
	if o, ok := b.orders[uid]; ok {
		o.Submit()
		b.eventEngine.BroadCast(o)
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

//commission 반영
func (b *DefaultBroker) transmit(o *order.Order) {
	o.Execute()
	b.eventEngine.BroadCast(o)
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

func (b *DefaultBroker) SetEventBroadCaster(e event.EventBroadcaster) {
	b.eventEngine = e
}
