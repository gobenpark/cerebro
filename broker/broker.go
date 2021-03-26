package broker

//go:generate mockgen -source=./broker.go -destination=./mock/mock_broker.go

import (
	"sync"
	"sync/atomic"
	"time"

	terr "github.com/gobenpark/trader/error"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/satori/go.uuid"
)

type Broker interface {
	Buy(code string, size int64, price float64) string
	Sell(code string, size int64, price float64) string
	Cancel(uuid string)
	Submit(uid string)
	GetPosition(code string) ([]position.Position, error)
	AddOrderHistory()
	SetFundHistory()
	CommissionInfo()
	SetCash(cash int64)
	SetEventBroadCaster(e event.Broadcaster)
}

type DefaultBroker struct {
	sync.RWMutex
	cash        int64
	commission  float64
	orders      map[string]*order.Order
	mu          sync.Mutex
	eventEngine event.Broadcaster
	positions   map[string][]position.Position
}

// NewBroker Init new broker with cash,commission
func NewBroker(cash int64, commission float64) *DefaultBroker {
	return &DefaultBroker{
		cash:       cash,
		commission: commission,
		orders:     make(map[string]*order.Order),
		positions:  make(map[string][]position.Position),
	}
}

func (b *DefaultBroker) Buy(code string, size int64, price float64) string {
	uid := uuid.NewV4().String()
	o := &order.Order{
		OType:     order.Buy,
		Code:      code,
		UUID:      uid,
		Size:      size,
		Price:     price,
		CreatedAt: time.Now(),
	}
	o.Submit()
	b.orders[o.UUID] = o
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
	b.Submit(o.UUID)
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
		b.positions[o.Code] = append(b.positions[o.Code], position.Position{
			Size:      o.Size,
			Price:     o.Price,
			CreatedAt: o.CreatedAt,
		})
		return
	}
}

func (b *DefaultBroker) GetPosition(code string) ([]position.Position, error) {
	if p, ok := b.positions[code]; ok {
		return p, nil
	}

	return nil, terr.ErrNotExistCode
}

func (b *DefaultBroker) SetCash(cash int64) {
	atomic.StoreInt64(&b.cash, cash)
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

func (b *DefaultBroker) SetEventBroadCaster(e event.Broadcaster) {
	b.eventEngine = e
}
