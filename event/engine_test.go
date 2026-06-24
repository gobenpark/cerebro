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

// TestEngine_StartWaitsForListenersBeforeReturning locks the shutdown barrier: a
// caller that joins Start must be guaranteed that every already-broadcast event
// has been processed, even by a listener that lags the dispatcher.
func TestEngine_StartWaitsForListenersBeforeReturning(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := event.NewEventEngine()
	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	var got []any
	done := make(chan struct{})
	go func() {
		eng.Start(ctx)
		close(done)
	}()

	eng.Register(ctx, &captureListener{fn: func(_ context.Context, e any) {
		time.Sleep(time.Millisecond) // a slow listener that lags the dispatcher
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	}})

	const n = 25
	for i := range n {
		eng.BroadCast(i)
	}

	// Cancel right after broadcasting; Start must not return until the slow listener
	// has drained every event already accepted.
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, got, n, "every broadcast must be processed before Start returns")
}

// TestEngine_DuplicateRegistrationDoesNotHangShutdown locks the fix for a
// duplicate Register orphaning a delivery worker: the shutdown barrier must not
// wait forever on a worker whose channel was overwritten and never closed.
func TestEngine_DuplicateRegistrationDoesNotHangShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := event.NewEventEngine()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		eng.Start(ctx)
		close(done)
	}()

	l := &captureListener{fn: func(context.Context, any) {}}
	eng.Register(ctx, l)
	eng.Register(ctx, l) // duplicate: must be a no-op, not a second orphaned worker

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel — a duplicate registration orphaned a worker")
	}
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
