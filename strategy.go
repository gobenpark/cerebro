package cerebro

import (
	"context"
	"fmt"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/container"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/strategy"
)

type stg struct {
}

func (s *stg) CandleType() strategy.CandleType {
	//TODO implement me
	panic("implement me")
}

func (s *stg) Next(ctx context.Context, broker *broker.Broker, c container.Container) error {
	fmt.Println(c.Candle(container.Min3))
	return nil
}

func (s *stg) NotifyOrder(o order.Order) {
	//TODO implement me
	panic("implement me")
}

func (s *stg) NotifyTrade() {
	//TODO implement me
	panic("implement me")
}

func (s *stg) NotifyCashValue(before, after int64) {
	//TODO implement me
	panic("implement me")
}

func (s *stg) NotifyFund() {
	//TODO implement me
	panic("implement me")
}

func (s *stg) Name() string {
	return "strategy"
}
