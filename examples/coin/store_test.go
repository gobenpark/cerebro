package coin

import (
	"context"
	"fmt"
	"testing"

	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/strategy"
)

func (Upbit) Order(ctx context.Context, o *order.Order) error {
	panic("implement me")
}

func (Upbit) Cancel(id string) error {
	panic("implement me")
}

func (Upbit) LoadHistory(ctx context.Context, code string, d time.Duration) ([]container.Candle, error) {
	panic("implement me")
}

func (Upbit) Uid() string {
	panic("implement me")
}

func (Upbit) Cash() int64 {
	panic("implement me")
}

func (Upbit) Commission() float64 {
	panic("implement me")
}

func (Upbit) Positions() []position.Position {
	panic("implement me")
}

func (Upbit) OrderState(ctx context.Context) (<-chan event.OrderEvent, error) {
	panic("implement me")
}

func (Upbit) OrderInfo(id string) (*order.Order, error) {
	panic("implement me")
}

type st struct {
}

func (s st) CandleType() strategy.CandleType {
	//TODO implement me
	panic("implement me")
}

func (s st) Next(broker broker.Broker, container container.Container2) error {

	sma := indicators.NewSma(15, 0)
	candles := container.Candles(3 * time.Minute)
	sma.Calculate(candles)
	if sma.PeriodSatisfaction() {
		datas := sma.Get()
		if len(datas) > 2 && (datas[len(datas)-1].Data > datas[len(datas)-2].Data) {
			fmt.Println(container.Code())
			fmt.Println(datas[len(datas)-1].Data, datas[len(datas)-1].Date)
			fmt.Println(datas[len(datas)-2].Data, datas[len(datas)-2].Date)
		}
	}

	//length := len(container.Candles(3 * time.Minute))
	//if length > 2 {
	//	if container.Candles(3 * time.Minute)[length-1].Close > container.Candles(3 * time.Minute)[length-2].Close {
	//		fmt.Println(container.Code())
	//		fmt.Println(container.Candles(3 * time.Minute)[length-1].Close, container.Candles(3 * time.Minute)[length-1].Date)
	//		fmt.Println(container.Candles(3 * time.Minute)[length-2].Close, container.Candles(3 * time.Minute)[length-2].Date)
	//		fmt.Println("---------------------------------------------")
	//	}
	//}
	return nil
}

func (s st) NotifyOrder(o *order.Order) {
	//TODO implement me
	panic("implement me")
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

func TestUpbit_Tick(t *testing.T) {
	store := NewStore()
	items := store.GetMarketItems()
	var codes []string
	for _, code := range items {
		codes = append(codes, code.Code)
	}

	c := cerebro.NewCerebro(
		cerebro.WithLive(),
		cerebro.WithStore(NewStore()),
		cerebro.WithTargetItem(codes...),
	)
	c.SetStrategy(st{})
	c.Start()
}
