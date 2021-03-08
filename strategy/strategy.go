package strategy

import (
	"context"

	"github.com/gobenpark/trader/broker"
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
	Broker broker.Broker
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
