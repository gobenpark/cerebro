package market

//go:generate mockgen -source=./market.go -destination=./mock/mock_market.go

import (
	"context"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

type (
	CandleType int

	TickEventHandler    func() []*item.Item
	AccountEventHandler func()
	OrderEventHandler   func()
)

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
	Stocks(ctx context.Context) []*item.Item
	Candles(ctx context.Context, code string, level CandleType) (indicator.Candles, error)
	Subscribe(event interface{}) error
	Order(ctx context.Context, o order.Order) error
	AccountPositions() []position.Position
	AccountBalance() int64
	Events(ctx context.Context) <-chan interface{}
	Commission() float64
}
