package strategy

import (
	"fmt"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
)

type Bighands struct {
	Broker broker.Broker
	indi   indicators.Indicator
}

func (s *Bighands) Next(broker broker.Broker, container domain.Container) {
	rsi := indicators.NewRsi(14)
	rsi.Calculate(container)
	fmt.Printf("%s:%f", container.Code(), rsi.Get()[0].Data)

	if len(rsi.Get()) != 0 && rsi.Get()[0].Data < 30 {
		fmt.Printf("%s is upper 30 rsi\n", container.Code())
	}
	broker.Buy(container.Code(), 10, 1000.0)

	//
	//if container.Values()[0].Close > container.Values()[1].Close {
	//	fmt.Println("value change more upper ")
	//}

	//broker.Buy("KRW-BTC", 10, 1)
}

func (s *Bighands) NotifyOrder(o *order.Order) {
	switch o.Status() {
	case order.Submitted:
		fmt.Printf("%s:%s\n", o.Code, "Submitted")
	case order.Expired:
		fmt.Println("expired")
	case order.Rejected:
		fmt.Println("rejected")
	case order.Canceled:
		fmt.Println("canceled")
	case order.Completed:
		fmt.Printf("%s:%s\n", o.Code, "Completed")
	case order.Partial:
		fmt.Println("partial")
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
