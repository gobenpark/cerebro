package market

//go:generate mockgen -source=./market.go -destination=./mock/mock_market.go

import (
	"context"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

type CandleType int

const (
	Min CandleType = iota + 1
	Min2
	Min3
	Min4
	Min5
	Day
	Week
)

type Market interface {
	MarketItems(ctx context.Context) []item.Item
	Candles(ctx context.Context, code string, level CandleType) (indicator.Candles, error)
	Tick(ctx context.Context, item ...item.Item) (<-chan indicator.Tick, error)
	UID() string
	Order(ctx context.Context, o order.Order) error
	AccountPositions() []position.Position
	AccountBalance() int64
	Events(ctx context.Context) <-chan MarketEvent
	Commission() float64
}
