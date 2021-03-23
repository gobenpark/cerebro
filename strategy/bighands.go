package strategy

import (
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
)

type Bighands struct {
	Broker domain.Broker
	indi   indicators.Indicator
}

func (s *Bighands) Next(broker domain.Broker, container domain.Container) {
	rsi := indicators.NewRsi(14)
	rsi.Calculate(container)
	if len(rsi.Get()) != 0 {
		fmt.Printf("%s %d\n", container.Code(), container.Level())
		fmt.Println(rsi.Get()[0])
	}
	broker.Buy(container.Code(), 10, 1000.0)

	//
	//if container.Values()[0].Close > container.Values()[1].Close {
	//	fmt.Println("value change more upper ")
	//}

	//broker.Buy("KRW-BTC", 10, 1)
}

func (s *Bighands) NotifyOrder(o *order.Order) {
	fmt.Println(o.Status)
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
