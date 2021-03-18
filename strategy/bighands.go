package strategy

import (
	"context"
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
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
	if s.indi == nil {
		s.indi = indicators.NewRsi(0)
	}
	if container.Values()[0].Code == "KRW-BORA" {
		s.indi.Calculate(container)
		fmt.Println(container.Values()[:14])
		fmt.Println(s.indi.Get()[0])
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

func (s *Bighands) Start(ctx context.Context, event chan event.Event) {
	for {
		select {
		case <-ctx.Done():
			break
		case e := <-event:
			fmt.Println(e)
		}
	}
}
