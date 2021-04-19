package main

import (
	"fmt"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
)

type Bighands struct {
	Broker broker.Broker
	indi   indicators.Indicator
}

func (s *Bighands) Next(broker *broker.Broker, container container.Container) {
	rsi := indicators.NewRsi(14)
	rsi.Calculate(container)
	fmt.Println(rsi.Get()[0])

	fmt.Println(broker.GetCash())

	obv := indicators.NewObv()

	obv.Calculate(container)
	fmt.Println(obv.Get()[0])
	fmt.Println(container.Code())

	sma := indicators.NewSma(20)
	sma.Calculate(container)
	fmt.Println(sma.Get()[0])
	fmt.Println(container.Code())
}

func (s *Bighands) NotifyOrder(o *order.Order) {
	switch o.Status() {
	case order.Submitted:
		fmt.Printf("%s:%s\n", o.Code, "Submitted")
		fmt.Println(o.ExecutedAt)
	case order.Expired:
		fmt.Println("expired")
		fmt.Println(o.ExecutedAt)
	case order.Rejected:
		fmt.Println("rejected")
		fmt.Println(o.ExecutedAt)
	case order.Canceled:
		fmt.Println("canceled")
		fmt.Println(o.ExecutedAt)
	case order.Completed:
		fmt.Printf("%s:%s\n", o.Code, "Completed")
		fmt.Println(o.ExecutedAt)
		fmt.Println(o.Price)
		fmt.Println(o.Code)
		fmt.Println(o.Size)
	case order.Partial:
		fmt.Println("partial")
		fmt.Println(o.ExecutedAt)
	}
}

func (s *Bighands) NotifyTrade() {
	panic("implement me")
}

func (s *Bighands) NotifyCashValue() {
	panic("implement me")
}

func (s *Bighands) NotifyFund() {
	panic("implement me")
}
