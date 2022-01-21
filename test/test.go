package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/item"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

type st struct {
}

func (s st) GetMarketItems() []item.Item {

	return []item.Item{
		{
			Code: "005930",
			Type: "kospi",
			Name: "삼성전자",
			Tag:  "모",
		},
		{
			Code: "005935",
			Type: "kospi",
			Name: "삼성전자",
			Tag:  "모",
		},
	}
}

func (s st) Candles(ctx context.Context, code string, c store.CandleType, value int) ([]container.Candle, error) {
	res, err := resty.New().R().Get(fmt.Sprintf("https://api.alphasquare.co.kr/data/v2/price/candle-history/%s?freq=day", code))
	if err != nil {
		return nil, err
	}
	var m [][]float64
	err = json.Unmarshal(res.Body(), &m)
	if err != nil {
		return nil, err
	}

	var datas []container.Candle
	for _, i := range m {
		datas = append(datas, container.Candle{
			Code:   code,
			Open:   i[1],
			High:   i[2],
			Low:    i[3],
			Close:  i[4],
			Volume: i[5],
			Date:   time.UnixMilli(int64(i[0])),
		})
	}

	return datas, nil
}

func (s st) TradeCommits(ctx context.Context, code string) ([]container.TradeHistory, error) {
	//TODO implement me
	panic("implement me")
}

func (s st) Tick(ctx context.Context, code string) (<-chan container.Tick, error) {
	//TODO implement me
	panic("implement me")
}

func (s st) Order(ctx context.Context, o *order.Order) error {

	fmt.Println(o)
	return nil
}

func (s st) Cancel(id string) error {
	//TODO implement me
	panic("implement me")
}

func (s st) Uid() string {
	//TODO implement me
	panic("implement me")
}

func (s st) Cash() int64 {
	//TODO implement me
	panic("implement me")
}

func (s st) Commission() float64 {
	//TODO implement me
	panic("implement me")
}

func (s st) Positions() []position.Position {
	//TODO implement me
	panic("implement me")
}

func (s st) OrderState(ctx context.Context) (<-chan event.OrderEvent, error) {
	//TODO implement me
	panic("implement me")
}

func (s st) OrderInfo(id string) (*order.Order, error) {
	//TODO implement me
	panic("implement me")
}

type strate struct {
}

func (s strate) CandleType() strategy.CandleType {
	//TODO implement me
	panic("implement me")
}

func (s strate) Next(broker broker.Broker, container container.Container) error {
	if container.Values()[0].Close > container.Values()[1].Close {
		fmt.Println(container.Values()[0])
		bd := indicators.NewRsi(15)
		bd.Calculate(container)
		lo, err := time.LoadLocation("Asia/Seoul")
		if err != nil {
			return err
		}
		for _, i := range bd.Get() {
			if i.Date.Equal(time.Date(2022, time.January, 20, 9, 0, 0, 0, lo)) {
				return broker.Order(context.Background(), container.Code(), 10, 10, order.Buy, order.Close)
			}
		}
	}
	return nil
}

func (s strate) NotifyOrder(o *order.Order) {
	//TODO implement me
	panic("implement me")
}

func (s strate) NotifyTrade() {
	//TODO implement me
	panic("implement me")
}

func (s strate) NotifyCashValue() {
	//TODO implement me
	panic("implement me")
}

func (s strate) NotifyFund() {
	//TODO implement me
	panic("implement me")
}

func main() {

	c := cerebro.NewCerebro(
		cerebro.WithStore(st{}),
	)

	c.SetStrategy(strate{})

	c.Start()

}
