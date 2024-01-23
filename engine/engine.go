package engine

import (
	"context"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
)

type Engine interface {
	Spawn(ctx context.Context, item []item.Item, tk <-chan indicator.Tick) error
	event.Listener
}
