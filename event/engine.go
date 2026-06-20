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
)

// listenerBuffer bounds how many events may queue per listener before the
// dispatch loop applies backpressure.
const listenerBuffer = 64

type Engine struct {
	broadcast  chan any
	Register   chan Listener
	Unregister chan Listener
	// listeners maps each registered listener to its private delivery queue.
	// Only Start touches this map, so it needs no lock.
	listeners map[Listener]chan any
}

func NewEventEngine() *Engine {
	return &Engine{
		broadcast:  make(chan any, 10),
		Register:   make(chan Listener, 2),
		Unregister: make(chan Listener, 1),
		listeners:  make(map[Listener]chan any),
	}
}

// Start runs the dispatch loop; it must run in its own goroutine. Each listener
// is fed by a dedicated worker goroutine so that a slow (or re-entrant) listener
// cannot block the loop or other listeners — a listener may safely BroadCast
// from within its own Listen.
func (e *Engine) Start(ctx context.Context) {
	defer func() {
		for cli, ch := range e.listeners {
			close(ch)
			delete(e.listeners, cli)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-e.broadcast:
			for _, ch := range e.listeners {
				select {
				case ch <- evt:
				case <-ctx.Done():
					return
				}
			}
		case cli := <-e.Register:
			if cli == nil {
				continue
			}
			ch := make(chan any, listenerBuffer)
			e.listeners[cli] = ch
			go deliver(ctx, cli, ch)
		case cli := <-e.Unregister:
			if cli == nil {
				continue
			}
			if ch, ok := e.listeners[cli]; ok {
				close(ch)
				delete(e.listeners, cli)
			}
		}
	}
}

// deliver feeds one listener its events in order until the queue is closed or
// the context is canceled.
func deliver(ctx context.Context, l Listener, ch <-chan any) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			l.Listen(ctx, evt)
		}
	}
}

func (e *Engine) BroadCast(evt any) {
	e.broadcast <- evt
}
