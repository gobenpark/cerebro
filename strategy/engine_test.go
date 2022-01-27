package strategy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
	"golang.org/x/exp/rand"
)

/*
Todo: strategy engine을 개별 전략마다 담당  store에서 받은 틱데이터나 기타 데이터를 engine에 저장한 압축등을 통해서 압축데이터가 있을시 next call
초기에 압축방법 또는 써야될 보조지표등을 engine에 설정.

*/

type SampleStrategy struct {
}

func (s SampleStrategy) CandleType() CandleType {
	panic("implement me")
}

func (s SampleStrategy) Next(broker broker.Broker, container container.Container) error {
	//TODO implement me
	panic("implement me")
}

func (s SampleStrategy) NotifyOrder(o *order.Order) {
	//TODO implement me
	panic("implement me")
}

func (s SampleStrategy) NotifyTrade() {
	//TODO implement me
	panic("implement me")
}

func (s SampleStrategy) NotifyCashValue() {
	//TODO implement me
	panic("implement me")
}

func (s SampleStrategy) NotifyFund() {
	//TODO implement me
	panic("implement me")
}

func TestNewEngine(t *testing.T) {

	bufcd := []container.Candle{}
	//bufbol := []indicators.Indicate{}

	ctx, cancel := context.WithTimeout(context.TODO(), 20*time.Second)
	ch := make(chan container.Candle, 100)

	defer cancel()

	go func() {
		timer := time.NewTicker(time.Second)
	Done:
		for {
			select {
			case <-ctx.Done():
				break Done
			case <-timer.C:
				ch <- container.Candle{
					Code:   "005930",
					Open:   rand.Float64(),
					High:   rand.Float64(),
					Low:    rand.Float64(),
					Close:  rand.Float64(),
					Volume: rand.Float64(),
					Date:   time.Now(),
				}
			}
		}
	}()

	//bol := make(chan container.Candle, 100)

	go func() {
		bolind := indicators.NewRsi(15)

	Done:
		for {
			select {
			case <-ctx.Done():
				break Done
			case cd := <-ch:
				bufcd = append(bufcd, cd)
				bolind.Calculate(bufcd)
				indi := bolind.Get()
				fmt.Println(indi)
			}
		}

	}()

	bk := broker.NewBroker(nil)
	eg := NewEngine(bk, SampleStrategy{}, ch)

	eg.Start(ctx)

	<-ctx.Done()
}
