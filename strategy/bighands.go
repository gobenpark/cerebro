package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
)

type Bighands struct {
	Broker domain.Broker
}

func (b *Bighands) Next(data map[string]map[time.Duration][]domain.Candle) {
	//b.Broker.Buy("KRW", 10, 1000)
	fmt.Println("next")
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

func (s *Bighands) Buy(code string, size int64, price float64) {
	s.Broker.Buy(code, size, price)
}
func (s *Bighands) Sell(code string, size int64, price float64) {
	s.Broker.Sell(code, size, price)
}
