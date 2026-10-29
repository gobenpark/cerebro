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
	"fmt"
	"log/slog"
	"sync"
)

// backlogWarnAt is the first backlog at which a mailbox logs that its listener
// is falling behind; each later warning fires at double the last.
const backlogWarnAt = 1024

// mailbox is one listener's unbounded FIFO delivery queue. The dispatch loop
// pushes without ever blocking, so a listener that broadcasts from inside its own
// Listen can never deadlock against a dispatcher stuck handing it a backlog. The
// price is that a persistently slow listener grows its queue instead of slowing
// producers; the backlog warnings make that visible. They are logged from the
// listener's own worker, never from the dispatch loop, so a slow log sink cannot
// stall dispatch either.
type mailbox struct {
	name string

	mu     sync.Mutex
	queue  []any
	closed bool
	// wake carries at most one pending signal that the queue changed.
	wake chan struct{}

	// warnAt is touched only by the worker (take), so it needs no lock.
	warnAt int
}

func newMailbox(l Listener) *mailbox {
	return &mailbox{
		name:   fmt.Sprintf("%T", l),
		warnAt: backlogWarnAt,
		wake:   make(chan struct{}, 1),
	}
}

// push appends evt and returns immediately.
func (m *mailbox) push(evt any) {
	m.mu.Lock()
	m.queue = append(m.queue, evt)
	m.mu.Unlock()
	m.signal()
}

// close stops the mailbox once whatever is already queued has been taken.
func (m *mailbox) close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.signal()
}

func (m *mailbox) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// take blocks until events are queued and returns all of them in order, or
// returns ok=false once the mailbox is closed and empty. It runs on the
// listener's worker, which is also where a large backlog gets reported.
func (m *mailbox) take() (batch []any, ok bool) {
	for {
		m.mu.Lock()
		if len(m.queue) > 0 {
			batch, m.queue = m.queue, nil
			m.mu.Unlock()
			m.reportBacklog(len(batch))
			return batch, true
		}
		closed := m.closed
		m.mu.Unlock()
		if closed {
			return nil, false
		}
		<-m.wake
	}
}

// reportBacklog warns when a taken batch reaches the next doubling threshold and
// re-arms the first threshold once the listener has caught up.
func (m *mailbox) reportBacklog(n int) {
	if n < backlogWarnAt {
		m.warnAt = backlogWarnAt
		return
	}
	if n >= m.warnAt {
		slog.Default().Warn("event listener falling behind", "listener", m.name, "queued", n)
		m.warnAt = 2 * n
	}
}
