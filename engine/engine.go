package engine

import (
	"context"

	"github.com/gobenpark/cerebro/event"
)

type Engine interface {
	// Spawn starts the engine's long-running goroutines. The items to trade are
	// carried by the engine's own configuration (e.g. strategy runners), not passed
	// in here.
	Spawn(ctx context.Context)
	// Wait blocks until the engine's long-running goroutines (started by Spawn)
	// have returned after context cancellation.
	Wait()
	event.Listener
}
