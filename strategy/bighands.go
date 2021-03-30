package strategy

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

func (s *Bighands) Next(broker broker.Broker, container container.Container) {
	rsi := indicators.NewRsi(14)
	rsi.Calculate(container)
	fmt.Println(rsi.Get()[0])

	sma := indicators.NewSma(20)
	sma.Calculate(container)
	fmt.Println(sma.Get()[0])

	if len(rsi.Get()) != 0 && rsi.Get()[0].Data < 30 {
		fmt.Printf("%s is upper 30 rsi\n", container.Code())
	}

	b := indicators.NewBollingerBand(20)
	b.Calculate(container)
	if len(b.Top) != 0 {
		fmt.Printf("top: %f\n", b.Top[0].Data)
		fmt.Printf("mid: %f\n", b.Mid[0].Data)
		fmt.Printf("bottom: %f\n", b.Bottom[0].Data)
	}
	broker.Buy(container.Code(), 10, 1000.0, order.Market)

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
