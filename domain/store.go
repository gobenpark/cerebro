//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go
package domain

import (
	"context"
	"time"
)

type Store interface {
	Order()
	Cancel()
	LoadHistory(ctx context.Context, d time.Duration) ([]Candle, error)
	LoadTick(ctx context.Context) (<-chan Tick, error)
	Uid() string
	Code() string
}
