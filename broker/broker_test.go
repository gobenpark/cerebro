package broker

import (
	"context"
	"fmt"
	"testing"
	"time"

	mock_event "github.com/gobenpark/trader/event/mock"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	mock_store "github.com/gobenpark/trader/store/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func HelperNewBroker(t *testing.T) (Broker, *mock_store.MockStore, *mock_event.MockBroadcaster) {
	t.Helper()

	ctrl := gomock.NewController(t)
	st := mock_store.NewMockStore(ctrl)
	evt := mock_event.NewMockBroadcaster(ctrl)
	broker := NewBroker(st, evt)
	return broker, st, evt
}

func TestBroker_GetCash(t *testing.T) {
	b, st, _ := HelperNewBroker(t)
	st.EXPECT().Cash().Return(int64(10))

	cash := b.GetCash()
	assert.Equal(t, int64(10), cash)
}

func TestBroker_Order(t *testing.T) {
	b, st, evt := HelperNewBroker(t)

	ctx := context.TODO()
	code := "code"
	size := int64(10)
	price := float64(1230)

	st.EXPECT().
		Order(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o *order.Order) error {
			if o.Code != code ||
				o.Price != price ||
				o.Size != size ||
				o.Action != order.Buy ||
				o.ExecType != order.Close {
				return fmt.Errorf("error expect %v", o)
			}
			return nil
		})
	evt.EXPECT().BroadCast(gomock.Any()).Do(func(e interface{}) {
		if o, ok := e.(order.Order); ok {
			if o.Code != code ||
				o.Price != price ||
				o.Size != size ||
				o.Action != order.Buy ||
				o.ExecType != order.Close {
				require.Fail(t, "error broadCast")
			}
		} else {
			require.Fail(t, "fail convert")
		}
	})

	err := b.Order(ctx, code, size, price, order.Buy, order.Close)
	require.NoError(t, err)
}

func TestBroker_GetPosition(t *testing.T) {
	b, st, _ := HelperNewBroker(t)

	st.EXPECT().Positions().Return([]position.Position{
		{
			Code:      "50912",
			Size:      10,
			Price:     1000,
			CreatedAt: time.Now(),
		},
	})
	positions := b.GetPosition()
	assert.Len(t, positions, 1)
}
