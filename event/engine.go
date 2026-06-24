/*
 *  Copyright 2021 The Cerebro Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package event

import (
	"context"
	"sync"
)

// listenerBuffer bounds how many events may queue per listener before the
// dispatch loop applies backpressure.
const listenerBuffer = 64

// registration hands a listener to the dispatch loop together with an ack channel
// that the loop closes once the listener is live. Register blocks on that ack, so
// a producer can register and then broadcast without racing its own registration.
type registration struct {
	listener Listener
	ack      chan struct{}
}

type Engine struct {
	broadcast chan any
	register  chan registration
	// listeners maps each registered listener to its private delivery queue.
	// Only Start touches this map, so it needs no lock.
	listeners map[Listener]chan any
	// deliverWg tracks the per-listener delivery goroutines so Start can wait for
	// them to finish draining on shutdown, making Start a barrier: once it returns,
	// every dispatched event has been processed by every listener.
	deliverWg sync.WaitGroup
}

func NewEventEngine() *Engine {
	return &Engine{
		broadcast: make(chan any, 10),
		register:  make(chan registration),
		listeners: make(map[Listener]chan any),
	}
}

// Start runs the dispatch loop; it must run in its own goroutine. Each listener
// is fed by a dedicated worker goroutine so that a slow (or re-entrant) listener
// cannot block the loop or other listeners — a listener may safely BroadCast
// from within its own Listen.
func (e *Engine) Start(ctx context.Context) {
	defer func() {
		// Stop the delivery queues, then wait for every worker to finish draining.
		// This makes Start a barrier: a caller that joins it (Cerebro.Shutdown) is
		// then guaranteed that all dispatched events have been processed.
		for cli, ch := range e.listeners {
			close(ch)
			delete(e.listeners, cli)
		}
		e.deliverWg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			// Flush events already queued in broadcast to the listeners so a shutdown
			// right after a broadcast doesn't drop the last notifications.
			e.drainToListeners()
			return
		case evt := <-e.broadcast:
			e.fanout(evt)
		case reg := <-e.register:
			// Register is idempotent: a listener that is already registered keeps its
			// existing queue and worker. A duplicate registration must not overwrite
			// the queue, or the first worker would be orphaned on a channel that is
			// never closed — and the shutdown barrier (deliverWg.Wait) would then wait
			// on it forever.
			if _, ok := e.listeners[reg.listener]; !ok {
				ch := make(chan any, listenerBuffer)
				e.listeners[reg.listener] = ch
				e.deliverWg.Go(func() {
					deliver(ctx, reg.listener, ch)
				})
			}
			// Signal Register that the listener is live and will receive every
			// broadcast from here on.
			close(reg.ack)
		}
	}
}

// fanout delivers evt to every listener's queue, blocking until each accepts.
// The workers drain continuously, so this applies backpressure rather than
// dropping events; a full queue slows the dispatcher instead of losing a fill.
func (e *Engine) fanout(evt any) {
	for _, ch := range e.listeners {
		ch <- evt
	}
}

// drainToListeners flushes the remaining broadcast queue to the listeners during
// shutdown, so events already accepted into broadcast are delivered before the
// queues are closed. The workers are still draining here, so the sends complete.
func (e *Engine) drainToListeners() {
	for {
		select {
		case evt := <-e.broadcast:
			e.fanout(evt)
		default:
			return
		}
	}
}

// deliver feeds one listener its events in order until its queue is closed,
// draining whatever is buffered first. Start closes the queue only after the
// dispatch loop has flushed every pending broadcast into it, so a worker that
// runs to completion has processed every event routed to its listener.
func deliver(ctx context.Context, l Listener, ch <-chan any) {
	for evt := range ch {
		l.Listen(ctx, evt)
	}
}

// Register adds l to the listener set and blocks until the dispatch loop has it
// live, returning false only if ctx is canceled first. Registering synchronously
// lets a producer guarantee that events it broadcasts immediately afterwards are
// delivered to l instead of racing ahead of the registration.
func (e *Engine) Register(ctx context.Context, l Listener) bool {
	if l == nil {
		return true
	}
	ack := make(chan struct{})
	select {
	case e.register <- registration{listener: l, ack: ack}:
	case <-ctx.Done():
		return false
	}
	select {
	case <-ack:
		return true
	case <-ctx.Done():
		return false
	}
}

func (e *Engine) BroadCast(evt any) {
	e.broadcast <- evt
}

// BroadCastContext sends evt to the dispatch loop, returning false if ctx is
// canceled before the send completes. Use this from producers that must not
// block once the engine is shutting down (the loop stops draining broadcast on
// cancellation).
func (e *Engine) BroadCastContext(ctx context.Context, evt any) bool {
	select {
	case e.broadcast <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}
