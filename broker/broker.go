package broker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/satori/go.uuid"
)

type Broker struct {
	sync.RWMutex
	sync.Once
	Cash        int64
	Commission  float64
	orders      map[string]*order.Order
	mu          sync.Mutex
	eventEngine event.Broadcaster
	positions   map[string][]position.Position
	Store       store.Store
}

// NewBroker Init new broker with cash,commission
func NewBroker() *Broker {
	return &Broker{
		orders:    make(map[string]*order.Order),
		positions: make(map[string][]position.Position),
	}
}

func (b *Broker) Buy(code string, size int64, price float64, exec order.ExecType) string {
	uid := uuid.NewV4().String()
	o := &order.Order{
		OType:     order.Buy,
		Code:      code,
		UUID:      uid,
		Size:      size,
		Price:     price,
		ExecType:  exec,
		CreatedAt: time.Now(),
	}
	b.orders[o.UUID] = o
	b.Submit(o)
	return uid
}

func (b *Broker) Sell(code string, size int64, price float64, exec order.ExecType) string {
	uid := uuid.NewV4().String()
	o := &order.Order{
		Code:     code,
		UUID:     uid,
		OType:    order.Sell,
		Size:     size,
		Price:    price,
		ExecType: exec,
	}
	b.orders[o.UUID] = o
	b.Submit(o)
	return uid
}

func (b *Broker) Cancel(uid string) {
	if o, ok := b.orders[uid]; ok {
		o.Cancel()
		b.eventEngine.BroadCast(o)
		return
	}
}

func (b *Broker) Submit(o *order.Order) {
	o.Submit()
	b.eventEngine.BroadCast(o)

	b.orders[o.UUID] = o
	if err := b.Store.Order(o); err != nil {
		o.Reject(err)
		b.eventEngine.BroadCast(o)
		return
	}

	return
}

func (b *Broker) Accept(oid string) {
	if o, ok := b.orders[oid]; ok {
		b.positions[o.Code] = append(b.positions[o.Code], position.Position{
			Code:      o.Code,
			Size:      o.Size,
			Price:     o.Price,
			CreatedAt: o.CreatedAt,
		})
		o.Complete()
		b.eventEngine.BroadCast(o)
		return
	}
}

func (b *Broker) GetPosition(code string) []position.Position {
	b.Do(func() {
		p := b.Store.Positions()
		for _, i := range p {
			b.positions[i.Code] = append(b.positions[i.Code], i)
		}
	})

	if p, ok := b.positions[code]; ok {
		return p
	}

	return nil
}

func (b *Broker) GetCash() int64 {
	return b.Store.Cash()
}

func (b *Broker) SetCash(cash int64) {
	atomic.StoreInt64(&b.Cash, cash)
}

func (b *Broker) AddOrderHistory() {
	panic("implement me")
}

func (b *Broker) SetFundHistory() {
	panic("implement me")
}

func (b *Broker) SetEventBroadCaster(e event.Broadcaster) {
	b.eventEngine = e
}

func (b *Broker) Listen(e interface{}) {
	if evt, ok := e.(event.OrderEvent); ok {
		switch evt.State {
		case "cancel":
			b.Cancel(evt.Oid)
		case "done":
			b.Accept(evt.Oid)
		case "wait":
			fmt.Println(b.positions)
			fmt.Println("wait")
		}
	}
}
