package store

import (
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	Store       Store
	Codes       []string
	EventEngine event.Broadcaster
}

func NewEngine() *Engine {
	return &Engine{}
}

func (s *Engine) Listen(e interface{}) {
	o, ok := e.(*order.Order)
	if !ok {
		return
	}

	if o.Status() == order.Submitted {
		if err := s.Store.Order(o.Code, o.OType, o.ExecType, o.Size, o.Price); err != nil {
			o.Reject(err)
			s.EventEngine.BroadCast(o)
			return
		}
		o.Complete()
		s.EventEngine.BroadCast(o)
	}

}
