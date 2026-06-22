package event_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/gobenpark/cerebro/event"
)

// captureListener adapts a function to event.Listener. It must be a pointer
// type: event.Engine uses listeners as map keys, and function types are
// unhashable (they would panic on insert).
type captureListener struct {
	fn func(context.Context, any)
}

func (l *captureListener) Listen(ctx context.Context, e any) { l.fn(ctx, e) }

func TestEngine_DeliversToRegisteredListener(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := event.NewEventEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go eng.Start(ctx)

	got := make(chan any, 1)
	eng.Register(ctx, &captureListener{fn: func(_ context.Context, e any) {
		select {
		case got <- e:
		default:
		}
	}})

	// Register blocks until the listener is live, but delivery still runs on a
	// worker goroutine, so poll until the broadcast has been forwarded.
	assert.Eventually(t, func() bool {
		eng.BroadCast("hello")
		select {
		case e := <-got:
			return e == "hello"
		case <-time.After(20 * time.Millisecond):
			return false
		}
	}, time.Second, 5*time.Millisecond)
}

// TestEngine_ListenerCanBroadcastWithoutDeadlock locks the fix for the
// synchronous-dispatch deadlock: a listener that calls BroadCast from inside its
// own Listen must not stall the dispatch loop. With per-listener worker
// goroutines the echo is delivered; with the old in-loop dispatch it deadlocks.
func TestEngine_ListenerCanBroadcastWithoutDeadlock(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := event.NewEventEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go eng.Start(ctx)

	echoed := make(chan struct{})
	var once sync.Once
	eng.Register(ctx, &captureListener{fn: func(_ context.Context, e any) {
		switch e {
		case "trigger":
			eng.BroadCast("echo") // re-entrant broadcast
		case "echo":
			once.Do(func() { close(echoed) })
		}
	}})

	assert.Eventually(t, func() bool {
		eng.BroadCast("trigger")
		select {
		case <-echoed:
			return true
		case <-time.After(20 * time.Millisecond):
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)
}
