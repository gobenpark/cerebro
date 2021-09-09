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

func (Upbit) LoadTick(ctx context.Context, code string) (<-chan container.Tick, error) {
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

func TestGetStockCodes(t *testing.T) {
	s := NewStore()
	items := s.GetStockCodes()

	for _, i := range items {
		fmt.Println(i)
	}
}
