/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */

package event

import "context"

type EventEngine struct {
	broadcast  chan Event
	Register   chan EventListener
	Unregister chan EventListener
	childEvent map[EventListener]bool
}

func NewEventEngine() *EventEngine {
	return &EventEngine{
		broadcast:  make(chan Event),
		Register:   make(chan EventListener),
		Unregister: make(chan EventListener),
		childEvent: make(map[EventListener]bool),
	}
}

func (e *EventEngine) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				break
			case evt := <-e.broadcast:
				for c := range e.childEvent {
					c.Listen(evt)
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

func (e *EventEngine) BroadCast(evt Event) {
	e.broadcast <- evt
}
