package coin

import (
	"context"
	"fmt"
	"testing"

	"time"

	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/stretchr/testify/assert"
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

func TestUpbit_GetMarketItems(t *testing.T) {
	s := NewStore()
	items := s.GetMarketItems()
	assert.NotEqual(t, 0, len(items))
}

func TestUpbit_Candles(t *testing.T) {
	s := NewStore()
	candle, err := s.Candles(context.TODO(), "KRW-BTC", store.DAY, 3)
	assert.NoError(t, err)
	fmt.Println(candle)
}

func TestUpbit_TradeCommits(t *testing.T) {
	s := NewStore()
	data, err := s.TradeCommits(context.TODO(), "KRW-BTC")
	assert.NoError(t, err)
	for _, i := range data {
		fmt.Println(i)
	}
}

func TestUpbit_Tick(t *testing.T) {
	s := NewStore()
	ch, err := s.Tick(context.TODO(), "KRW-BTC")
	assert.NoError(t, err)
	for i := range ch {
		fmt.Println(i)
	}
}