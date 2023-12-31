package store

//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go

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

type Store interface {
	//GetMarketItems get all market item
	MarketItems(ctx context.Context) []item.Item
	//Candles get level(min) candles level only can be minute
	Candles(ctx context.Context, code string, level CandleType) (indicator.Candles, error)

	Tick(ctx context.Context, item ...item.Item) (<-chan indicator.Tick, error)

	Order(ctx context.Context, o order.Order) error
	Cancel(o order.Order) error
	UID() string
	Cash() int64
	Commission() float64
	Positions() map[string]position.Position
	Events() <-chan interface{}
}
