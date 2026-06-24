package market

//go:generate mockgen -source=./market.go -destination=./mock/mock_market.go

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

type (
	CandleType int

	// TickEventHandler returns the items whose realtime tick feed should be
	// subscribed. It is passed to Subscribe.
	TickEventHandler func() []*item.Item
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
	Subscribe(ctx context.Context, handler TickEventHandler) error
	Order(ctx context.Context, o order.Order) error
	AccountPositions(ctx context.Context) []position.Position
	AccountBalance(ctx context.Context) decimal.Decimal
	Events(ctx context.Context) <-chan any
	// Commission is the percentage fee applied to an order's value. It must be a
	// cheap, non-blocking accessor (a cached/constant value) — it is read inside the
	// broker's lock and on every fill, so it takes no context and must not do I/O.
	Commission() decimal.Decimal
}
