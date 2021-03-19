package strategy

import (
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/indicators"
)

type Bighands struct {
	Broker domain.Broker
	indi   indicators.Indicator
}

func (s *Bighands) Indicators() []domain.Indicator {
	return []domain.Indicator{}
}

func (s *Bighands) Next(broker domain.Broker, container domain.Container) {
	rsi := indicators.NewRsi(14)
	rsi.Calculate(container)
	if len(rsi.Get()) != 0 {
		fmt.Println(container.Code())
		fmt.Println(rsi.Get()[0])
	}

	//
	//if container.Values()[0].Close > container.Values()[1].Close {
	//	fmt.Println("value change more upper ")
	//}

	//broker.Buy("KRW-BTC", 10, 1)
}

func (s *Bighands) NotifyOrder() {
	panic("implement me")
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
