package store

//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go

import (
	"context"
	"time"

	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
)

type Store interface {
	Order(code string, ot order.OType, size int64, price float64) error
	Cancel(id string) error
	LoadHistory(ctx context.Context, code string, d time.Duration) ([]container.Candle, error)
	LoadTick(ctx context.Context, code string) (<-chan container.Tick, error)
	Uid() string
}
