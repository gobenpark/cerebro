package broker

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/gobenpark/trader/event"
	mock_event "github.com/gobenpark/trader/event/mock"
	"github.com/gobenpark/trader/log"
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
	log := log.NewZapLogger()
	broker := NewBroker(log, st, evt)
	return broker, st, evt
}

func TestBroker_GetCash(t *testing.T) {
	b, st, _ := HelperNewBroker(t)

	st.EXPECT().Cash().Return(int64(10))

	cash := b.Cash()
	assert.Equal(t, int64(10), cash)
}

func TestBroker_GetPosition(t *testing.T) {
	b, st, _ := HelperNewBroker(t)

	input := position.Position{
		Code:      "50912",
		Size:      10,
		Price:     1000,
		CreatedAt: time.Now(),
	}

	st.EXPECT().Positions().Return(map[string]position.Position{
		"50912": input,
	})
	position := b.Positions()
	require.True(t, reflect.DeepEqual(position["50912"], input))
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

	st.EXPECT().Positions().Return(map[string]position.Position{code: {
		Code:      code,
		Size:      size,
		Price:     price,
		CreatedAt: time.Now(),
	}})

	st.EXPECT().Cash().Return(int64(10))

	evt.EXPECT().BroadCast(gomock.Any()).Do(func(e interface{}) {
		switch o := e.(type) {
		case *order.Order:
			require.Equal(t, order.Completed, o.Status())
			if o.Code != code ||
				o.Price != price ||
				o.Size != size ||
				o.Action != order.Buy ||
				o.ExecType != order.Close {
				require.Fail(t, "error broadCast")
			}
		case event.CashEvent:
			require.Equal(t, o.After, int64(10))
		default:
			require.Failf(t, "invalid type of order pointer", "%#v", e)
		}
	}).After(evt.EXPECT().BroadCast(gomock.Any()).Do(func(e interface{}) {
		switch o := e.(type) {
		case *order.Order:
			require.Equal(t, order.Submitted, o.Status())
			if o.Code != code ||
				o.Price != price ||
				o.Size != size ||
				o.Action != order.Buy ||
				o.ExecType != order.Close {
				require.Fail(t, "error broadCast")
			}
		case event.CashEvent:
			require.Equal(t, o.After, int64(10))
		default:
			require.Failf(t, "invalid type of order pointer", "%#v", e)
		}
	}))

	b.Order(ctx, code, size, price, order.Buy, order.Close)
	<-time.After(time.Second)
}

func TestBroker_Order_Reject(t *testing.T) {
	b, st, evt := HelperNewBroker(t)

	ctx := context.TODO()
	code := "code"
	size := int64(10)
	price := float64(1230)

	st.EXPECT().
		Order(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("error"))

	st.EXPECT().Positions().Return(map[string]position.Position{code: {
		Code:      code,
		Size:      size,
		Price:     price,
		CreatedAt: time.Now(),
	}})

	st.EXPECT().Cash().Return(int64(10))

	evt.EXPECT().BroadCast(gomock.Any()).Do(func(e interface{}) {
		switch o := e.(type) {
		case *order.Order:
			require.Equal(t, order.Rejected, o.Status())

			if o.Code != code ||
				o.Price != price ||
				o.Size != size ||
				o.Action != order.Buy ||
				o.ExecType != order.Close {
				require.Fail(t, "error broadCast")
			}
		case event.CashEvent:
			require.Equal(t, o.After, int64(10))
		default:
			require.Failf(t, "invalid type of order pointer", "%#v", e)
		}
	}).After(evt.EXPECT().BroadCast(gomock.Any()).Do(func(e interface{}) {
		switch o := e.(type) {
		case *order.Order:
			require.Equal(t, order.Submitted, o.Status())

			if o.Code != code ||
				o.Price != price ||
				o.Size != size ||
				o.Action != order.Buy ||
				o.ExecType != order.Close {
				require.Fail(t, "error broadCast")
			}
		case event.CashEvent:
			require.Equal(t, o.After, int64(10))
		default:
			require.Failf(t, "invalid type of order pointer", "%#v", e)
		}
	}))

	b.Order(ctx, code, size, price, order.Buy, order.Close)
	<-time.After(time.Second)
}
