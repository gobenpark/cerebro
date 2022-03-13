package main

import (
	"fmt"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/strategy"
)

type st struct {
}

func (s st) CandleType() strategy.CandleType {
	panic("implement me")
}

func (s st) Next(broker broker.Broker, container container.Container2) error {

	if container.Code() == "KRW-WAVES" {
		sma := indicators.NewSma(15, 0)
		sma.Calculate(container.Candles(time.Minute))

		if sma.PeriodSatisfaction() {
			fmt.Println(sma.Get())
		}
	}

	return nil
}

func (s st) NotifyOrder(o order.Order) {
	switch o.Status() {
	case order.Accepted:
		fmt.Println("order accept")
	case order.Completed:
		fmt.Println("order completed")
	case order.Created:
		fmt.Println("order created")
	case order.Canceled:
		fmt.Println("order canceled")
	case order.Expired:
		fmt.Println("order exired")
	case order.Rejected:
		fmt.Println("order rejected")
	}
}

func (s st) NotifyTrade() {
	//TODO implement me
	panic("implement me")
}

func (s st) NotifyCashValue(before, after int64) {
	fmt.Printf("notify cash before %d, after %d\n", before, after)
}

func (s st) NotifyFund() {
	//TODO implement me
	panic("implement me")
}
