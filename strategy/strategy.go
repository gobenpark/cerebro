package strategy

import (
	"context"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
)

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

type Strategy interface {
	Next()

	NotifyOrder()
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
	Start(ctx context.Context, event chan event.Event)
	Buy()
	Sell()
}

type DefaultStrategy struct {
	Broker domain.Broker
}

func (s *DefaultStrategy) Next() {
	panic("implement me")
}

func (s *DefaultStrategy) NotifyOrder() {
	panic("implement me")
}

func (s *DefaultStrategy) NotifyTrade() {
	panic("implement me")
}

func (s *DefaultStrategy) NotifyCashValue() {
	panic("implement me")
}

func (s *DefaultStrategy) NotifyFund() {
	panic("implement me")
}

func (s *DefaultStrategy) Start(ctx context.Context, event chan event.Event) {
	for {
		select {
		case <-ctx.Done():
			break
		case <-event:

		}
	}
}

func (s *DefaultStrategy) Buy(code string, size int64, price float64) {
	s.Broker.Buy(code, size, price)
}
func (s *DefaultStrategy) Sell(code string, size int64, price float64) {
	s.Broker.Sell(code, size, price)
}
