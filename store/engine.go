package store

import (
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	Stores      map[string]Store
	Mapper      map[string][]string
	EventEngine event.Broadcaster
}

func NewEngine() *Engine {
	return &Engine{
		Stores: make(map[string]Store),
		Mapper: make(map[string][]string),
	}
}

func (s *Engine) Listen(e interface{}) {
	o, ok := e.(*order.Order)
	if !ok {
		return
	}

	if o.Status() == order.Submitted {
		for _, store := range s.Stores {
			if err := store.Order(o.Code, o.OType, o.Size, o.Price); err != nil {
				o.Reject(err)
				s.EventEngine.BroadCast(o)
				continue
			}
			o.Complete()
			s.EventEngine.BroadCast(o)
		}
	}
}
