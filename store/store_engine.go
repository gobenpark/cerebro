package store

import (
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/order"
)

type StoreEngine struct {
	store domain.Store
}

func (s *StoreEngine) Listen(e interface{}) {
	o, ok := e.(*order.Order)
	if !ok {
		return
	}

	if o.Status == order.Excuting {
		if err := s.store.Order(o.Code, func() domain.OrderType {
			switch o.OType {
			case order.Buy:
				return domain.Buy
			case order.Sell:
				return domain.Sell
			}
			return 0
		}(), o.Size, o.Price); err != nil {

		}
	}
}
