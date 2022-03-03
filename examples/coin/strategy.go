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
			if _, ok := broker.Positions()[container.Code()]; ok {

			} else {
				broker.Order(context.Background(), container.Code(), 10, candles[len(candles)-1].Close, order.Buy, order.Limit)
				//if errors.Is(err, error2.ErrNotEnoughMoney) {
				//	return nil
				//} else if err != nil {
				//	return err
				//}
				fmt.Println("position", broker.Positions())
			}
		}
	}

	return nil
}

func (s st) NotifyOrder(o order.Order) {
	switch o.Status() {
	case order.Accepted:
		fmt.Println("order accept")
	case order.Completed:
		fmt.Println("order completed")
	case order.Created:
		fmt.Println("order created")
	case order.Canceled:
		fmt.Println("order canceled")
	case order.Expired:
		fmt.Println("order exired")
	case order.Rejected:
		fmt.Println("order rejected")
	}
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
