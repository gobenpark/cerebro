package domain

import "context"

type Store interface {
	LoadHistory(ctx context.Context, code string) ([]Candle, error)
	LoadTick(ctx context.Context, code string) (<-chan Tick, error)
}
