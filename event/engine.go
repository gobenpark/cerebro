/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
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
		broadcast:  make(chan interface{}),
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
				if _, ok := e.childEvent[cli]; ok {
					delete(e.childEvent, cli)
				}
			}
		}
	}()
}

func (e *Engine) BroadCast(evt interface{}) {
	e.broadcast <- evt
}
