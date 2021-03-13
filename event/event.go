package event

import (
	"context"
	"time"
)

type EventType int

type Event struct {
	UUID         string
	EventID      EventType
	EventMessage string
	Date         time.Time
}

var (
	OrderSubmit = func() Event {
		return Event{UUID: "", EventID: 1, EventMessage: "Order Submit", Date: time.Now()}
	}
)

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
