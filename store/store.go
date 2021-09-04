package store

//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go

import (
	"context"
	"time"

	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
)

type Store interface {
	GetStockCodes()
	Order(o *order.Order) error
	Cancel(id string) error
	LoadHistory(ctx context.Context, code string, d time.Duration) ([]container.Candle, error)
	LoadTick(ctx context.Context, code string) (<-chan container.Tick, error)
	Uid() string
	Cash() int64
	Commission() float64
	Positions() []position.Position
	OrderState(ctx context.Context) (<-chan event.OrderEvent, error)
	OrderInfo(id string) (*order.Order, error)
}
