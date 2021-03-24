package strategy

import (
	"context"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	broker.Broker
	sts []Strategy
}

func (s *Engine) Start(ctx context.Context, data chan domain.Container, sts []Strategy) {
	s.sts = sts
	go func() {
		for i := range data {
			for _, strategy := range sts {
				strategy.Next(s.Broker, i)
			}
		}
	}()
}

func (s *Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case *order.Order:
		for _, strategy := range s.sts {
			strategy.NotifyOrder(et)
		}
	}
}
