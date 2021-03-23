package store

import (
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	Stores      []domain.Store
	EventEngine event.EventBroadcaster
}

func (s *Engine) Listen(e interface{}) {
	o, ok := e.(*order.Order)
	if !ok {
		return
	}

	if o.Status() == order.Submitted {
		for _, store := range s.Stores {
			if err := store.Order(o.Code, func() domain.OrderType {
				switch o.OType {
				case order.Buy:
					return domain.Buy
				case order.Sell:
					return domain.Sell
				}
				return 0
			}(), o.Size, o.Price); err != nil {
				o.Reject(err)
				s.EventEngine.BroadCast(o)
				continue
			}
			o.Complete()
			s.EventEngine.BroadCast(o)
		}
	}
}
