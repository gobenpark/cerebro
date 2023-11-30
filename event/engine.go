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

type Engine struct {
	broadcast  chan interface{}
	Register   chan Listener
	Unregister chan Listener
	childEvent map[Listener]bool
}

func NewEventEngine() *Engine {
	return &Engine{
		broadcast:  make(chan interface{}, 10),
		Register:   make(chan Listener, 2),
		Unregister: make(chan Listener, 1),
		childEvent: make(map[Listener]bool),
	}
}

// Start event engine start function need goroutine
func (e *Engine) Start(ctx context.Context) {
Done:
	for {
		select {
		case <-ctx.Done():
			break Done
		case evt := <-e.broadcast:
			for c := range e.childEvent {
				go c.Listen(evt)
			}
		case cli := <-e.Register:
			e.childEvent[cli] = true
		case cli := <-e.Unregister:
			delete(e.childEvent, cli)
		}
	}
}

func (e *Engine) BroadCast(evt interface{}) {
	e.broadcast <- evt
}
