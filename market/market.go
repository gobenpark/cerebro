package market

//go:generate mockgen -source=./market.go -destination=./mock/mock_market.go

import (
	"context"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
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
}
