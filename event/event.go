package event

import (
	"time"
)

type (
	TradeEvent interface {
		Event() EventType
		EventID() string
	}
)

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
