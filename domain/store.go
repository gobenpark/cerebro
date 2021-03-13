//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go
package domain

import "context"

type Store interface {
	LoadHistory(ctx context.Context) ([]Candle, error)
	LoadTick(ctx context.Context) (<-chan Tick, error)
}
