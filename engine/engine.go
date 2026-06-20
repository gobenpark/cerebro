package engine

import (
	"context"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/item"
)

type Engine interface {
	Spawn(ctx context.Context, item []*item.Item)
	// Wait blocks until the engine's long-running goroutines (started by Spawn)
	// have returned after context cancellation.
	Wait()
	event.Listener
}
