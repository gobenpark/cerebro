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
			if broker.Position(container.Code()) != nil {
				fmt.Println("exist position")
				fmt.Println(broker.Position(container.Code()))
			} else {
				if err := broker.Order(context.Background(), container.Code(), 10, candles[len(candles)-1].Close, order.Buy, order.Limit); err != nil {
					return err
				}
				fmt.Println("not exist so buy ")
			}
		}
	}

	return nil
}

func (s st) NotifyOrder(o order.Order) {
	fmt.Println("notify order", o)
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
