package strategy

import (
	"context"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	broker.Broker
	Sts []Strategy
}

func (s *Engine) Start(ctx context.Context, data chan container.Container) {
	go func() {
		for i := range data {
			for _, strategy := range s.Sts {
				go strategy.Next(s.Broker, i)
			}
		}
	}()
}

func (s *Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case *order.Order:
		for _, strategy := range s.Sts {
			go strategy.NotifyOrder(et)
		}
	}
}
