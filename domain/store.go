package domain

import "context"

type Store interface {
	LoadHistory(ctx context.Context) ([]Candle, error)
	LoadTick(ctx context.Context) (<-chan Tick, error)
}
