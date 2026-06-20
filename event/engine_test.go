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
	eng.Register <- &captureListener{fn: func(_ context.Context, e any) {
		select {
		case got <- e:
		default:
		}
	}}

	// Registration is processed asynchronously by Start, so retry the broadcast
	// until the listener is wired up and delivers.
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
	eng.Register <- &captureListener{fn: func(_ context.Context, e any) {
		switch e {
		case "trigger":
			eng.BroadCast("echo") // re-entrant broadcast
		case "echo":
			once.Do(func() { close(echoed) })
		}
	}}

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
