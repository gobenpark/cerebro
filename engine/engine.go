package engine

import (
	"context"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/item"
)

type Engine interface {
	Spawn(ctx context.Context, item []item.Item) error
	event.Listener
}
