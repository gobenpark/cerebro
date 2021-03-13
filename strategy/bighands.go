package strategy

import (
	"context"
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
)

type Bighands struct {
	Broker domain.Broker
}

func (s *Bighands) Next(broker domain.Broker, container domain.Container) {
	fmt.Println(container.Values("KRW-BTC")[0])
	broker.Buy("KRW-BTC", 10, 1)

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
