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

// TestEngine_ListenerCanBroadcastWhileBackloggedWithoutDeadlock locks the fix for
// the backlog deadlock seen live at market open: a listener stuck on one event
// while a burst arrives, which then broadcasts from inside Listen. With bounded
// queues the dispatcher blocked handing the burst to that very listener, so the
// burst producer (the market feed) and the re-entrant broadcast both hung.
func TestEngine_ListenerCanBroadcastWhileBackloggedWithoutDeadlock(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := event.NewEventEngine()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		eng.Start(ctx)
		close(done)
	}()

	busy := make(chan struct{})
	release := make(chan struct{})
	echoed := make(chan struct{})
	eng.Register(ctx, &captureListener{fn: func(ctx context.Context, e any) {
		switch e {
		case "fill":
			close(busy)
			<-release
			eng.BroadCastContext(ctx, "notify") // re-entrant, like broker.notifyOrder
		case "notify":
			close(echoed)
		}
	}})
	var releaseOnce sync.Once
	releaseListener := func() { releaseOnce.Do(func() { close(release) }) }
	defer func() {
		releaseListener()
		cancel()
		waitClosed(t, done, "Start did not return after cancel")
	}()

	eng.BroadCast("fill")
	waitClosed(t, busy, "listener never received the first event")

	burstDone := broadcastInts(ctx, eng, 1000)
	waitClosed(t, burstDone, "broadcast blocked behind a busy listener")

	releaseListener()
	waitClosed(t, echoed, "re-entrant broadcast from a backlogged listener never arrived")
}

// TestEngine_PreservesOrderForBackloggedListener locks per-listener FIFO across a
// backlog far deeper than any fixed buffer: fills and order updates must reach the
// broker in the order they were broadcast.
func TestEngine_PreservesOrderForBackloggedListener(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := event.NewEventEngine()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		eng.Start(ctx)
		close(done)
	}()

	release := make(chan struct{})
	var got []int
	eng.Register(ctx, &captureListener{fn: func(_ context.Context, e any) {
		<-release
		if i, ok := e.(int); ok {
			got = append(got, i)
		}
	}})

	const n = 5000
	burstDone := broadcastInts(ctx, eng, n)
	select {
	case <-burstDone:
	case <-time.After(2 * time.Second):
		t.Error("broadcast blocked behind a busy listener")
	}
	close(release)
	cancel()
	waitClosed(t, done, "Start did not return after cancel")

	want := make([]int, n)
	for i := range want {
		want[i] = i
	}
	assert.Equal(t, want, got)
}

// broadcastInts broadcasts 0..n-1 from its own goroutine and closes the returned
// channel once every send is accepted or ctx is canceled.
func broadcastInts(ctx context.Context, eng *event.Engine, n int) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range n {
			if !eng.BroadCastContext(ctx, i) {
				return
			}
		}
	}()
	return done
}

// waitClosed fails the test if ch is not closed within a bounded time, so a
// deadlock regression reports instead of hanging the run.
func waitClosed(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
	}
}
