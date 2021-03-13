package strategy

import (
	"context"
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
)

type StrategyEngine struct {
	E chan event.Event
	domain.Broker
}

func (s *StrategyEngine) Start(ctx context.Context, data chan domain.Container, sts []domain.Strategy) {
	go func() {
		for i := range data {
			for _, strategy := range sts {
				strategy.Next(s.Broker, i)
			}
		}
	}()
}

func (s *StrategyEngine) Listen(e event.Event) {
	fmt.Println(e)
}
