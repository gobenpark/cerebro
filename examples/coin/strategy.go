package coin

import (
	"context"
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

	sma := indicators.NewSma(3, 0)
	candles := container.Candles(3 * time.Second)
	sma.Calculate(candles)
	if sma.PeriodSatisfaction() {
		datas := sma.Get()
		if len(datas) > 2 && (datas[len(datas)-1].Data > datas[len(datas)-2].Data) {
			//fmt.Println(container.Code())
			//fmt.Println(datas[len(datas)-1].Data, datas[len(datas)-1].Date)
			//fmt.Println(datas[len(datas)-2].Data, datas[len(datas)-2].Date)

			if len(broker.GetPosition(container.Code())) == 0 {
				broker.Order(context.Background(), container.Code(), 10, candles[len(candles)-1].Close, order.Buy, order.Limit)
			}
		}
	}

	return nil
}

func (s st) NotifyOrder(o *order.Order) {

	fmt.Println(o.Code)
	fmt.Println(o.Status())
	fmt.Println("order success")
}

func (s st) NotifyTrade() {
	//TODO implement me
	panic("implement me")
}

func (s st) NotifyCashValue() {
	//TODO implement me
	panic("implement me")
}

func (s st) NotifyFund() {
	//TODO implement me
	panic("implement me")
}
