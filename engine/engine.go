package engine

import (
	"context"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
)

type Engine interface {
	Spawn(ctx context.Context, tk <-chan indicator.Tick, item []item.Item) error
}
