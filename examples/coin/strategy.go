package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	bk "github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/log/v1"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/strategy"
)

type st struct {
}

func (s st) CandleType() strategy.CandleType {
	panic("implement me")
}

func (s st) Next(broker bk.Broker, container container.Container2) error {

	if container.Code() == "KRW-WAVES" {

		sma5 := indicators.NewSma(3, 0)
		sma5.Calculate(container.Candles(3 * time.Minute))

		sma10 := indicators.NewSma(5, 0)
		sma10.Calculate(container.Candles(time.Minute))

		bollinger := indicators.NewBollingerBand(20)
		bollinger.Calculate(container.Candles(3 * time.Minute))

		if bollinger.PeriodSatisfaction() {
			fmt.Println(bollinger.Top[len(bollinger.Top)-1])
		}

		if sma10.PeriodSatisfaction() {
			a := sma10.Get()
			data1 := a[len(a)-1]

			b := sma5.Get()
			data2 := b[len(b)-1]

			if p, ok := broker.Position(container.Code()); !ok {
				if data1.Data > data2.Data {
					if err := broker.OrderCash(context.Background(), container.Code(), broker.Cash(), container.CurrentPrice(), order.Buy, order.Limit); err != nil {
						log.Error(err)
					}
				}
			} else {
				if data1.Data < data2.Data {
					if err := broker.Order(context.Background(), container.Code(), p.Size, container.CurrentPrice(), order.Sell, order.Limit); err != nil && !errors.Is(err, bk.PositionExists) {
						log.Error(err)
					}

				}
			}
		}
	}

	return nil
}

func (s st) NotifyOrder(o order.Order) {
	//switch o.Status() {
	//case order.Accepted:
	//	fmt.Println("order accept")
	//case order.Completed:
	//	fmt.Println("order completed")
	//case order.Created:
	//	fmt.Println("order created")
	//case order.Canceled:
	//	fmt.Println("order canceled")
	//case order.Expired:
	//	fmt.Println("order exired")
	//case order.Rejected:
	//	fmt.Println("order rejected")
	//}
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
