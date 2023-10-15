package store

//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go

import (
	"context"
	"time"

	"github.com/gobenpark/cerebro/indicators"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

type CandleType int

const (
	MIN CandleType = iota
	DAY
	WEEK
)

type Store interface {
	//GetMarketItems get all market item
	MarketItems(ctx context.Context) []item.Item
	//Candles get level(min) candles level only can be minute
	Candles(ctx context.Context, code string, level time.Duration) (indicators.Candles, error)

	Tick(ctx context.Context, item ...item.Item) (<-chan indicators.Tick, error)

	Order(ctx context.Context, o order.Order) error
	Cancel(o order.Order) error
	UID() string
	Cash() int64
	Commission() float64
	Positions() map[string]position.Position
	//OrderState(ctx context.Context) (<-chan event.OrderEvent, error)
}
