/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
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
		Register:   make(chan Listener),
		Unregister: make(chan Listener),
		childEvent: make(map[Listener]bool),
	}
}

func (e *Engine) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				break
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
	}()
}

func (e *Engine) BroadCast(evt interface{}) {
	e.broadcast <- evt
}
