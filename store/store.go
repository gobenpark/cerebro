package store

//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go

import (
	"context"

	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/item"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
)

type CandleType int

const (
	MIN CandleType = iota
	DAY
	WEEK
)

type Store interface {
	//GetMarketItems get all market item
	GetMarketItems() []item.Item

	Candles(ctx context.Context, code string, c CandleType, value int) ([]container.Candle, error)

	TradeCommits(ctx context.Context, code string) ([]container.TradeHistory, error)

	Tick(ctx context.Context, code string) (<-chan container.Tick, error)

	Order(ctx context.Context, o *order.Order) error
	Cancel(id string) error
	Uid() string
	Cash() int64
	Commission() float64
	Positions() []position.Position
	OrderState(ctx context.Context) (<-chan event.OrderEvent, error)
	OrderInfo(id string) (*order.Order, error)
}
