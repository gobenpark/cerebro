package broker

import (
	"errors"
	"testing"
	"time"

	mock_event "github.com/gobenpark/trader/event/mock"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	mock_store "github.com/gobenpark/trader/store/mock"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker()
	require.NotNil(t, b)
}

func TestDefaultBroker_Buy(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockBroadcaster(ctrl)
	store := mock_store.NewMockStore(ctrl)

	b := NewBroker()
	b.Store = store
	b.eventEngine = e

	input := &order.Order{
		OType: order.Buy,
		Code:  "testcode",
		UUID:  uuid.NewV4().String(),
		Size:  1,
		Price: 1,
	}
	input.Submit()
	e.EXPECT().BroadCast(gomock.AssignableToTypeOf(input)).Times(2)
	store.EXPECT().Order(gomock.AssignableToTypeOf(input)).Times(1)
	result := b.Buy("testcode", 1, 1, order.Market)
	assert.NotNil(t, result)
}

func TestDefaultBroker_Sell(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockBroadcaster(ctrl)
	store := mock_store.NewMockStore(ctrl)
	b := NewBroker()
	b.eventEngine = e
	b.Store = store

	input := &order.Order{
		OType: order.Buy,
		Code:  "testcode",
		UUID:  uuid.NewV4().String(),
		Size:  1,
		Price: 1,
	}
	input.Submit()

	e.EXPECT().BroadCast(gomock.AssignableToTypeOf(input)).Times(2)
	store.EXPECT().Order(gomock.AssignableToTypeOf(input)).Times(1)
	result := b.Sell("testcode", 1, 1, order.Limit)
	assert.NotNil(t, result)
}

func TestBroker_Submit(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockBroadcaster(ctrl)
	store := mock_store.NewMockStore(ctrl)

	b := NewBroker()
	b.SetEventBroadCaster(e)
	b.Store = store

	uuid := uuid.NewV4().String()
	input := &order.Order{
		OType:    0,
		ExecType: 0,
		Code:     "code",
		UUID:     uuid,
		Size:     10,
		Price:    21,
	}
	isFalse := false

	store.EXPECT().Order(input).DoAndReturn(func(_ *order.Order) error {
		if !isFalse {
			return nil
		}
		return errors.New("error!")
	}).AnyTimes()
	e.EXPECT().BroadCast(gomock.AssignableToTypeOf(input)).AnyTimes()
	b.Submit(input)

	assert.Equal(t, float64(21), b.orders[uuid].Price)
	assert.Equal(t, int64(10), b.orders[uuid].Size)

	t.Run("submit reject", func(t *testing.T) {
		isFalse = true
		b.Submit(input)
		assert.Equal(t, order.Rejected, input.Status())
	})

}

func TestBroker_Accept(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockBroadcaster(ctrl)
	store := mock_store.NewMockStore(ctrl)

	b := NewBroker()
	b.SetEventBroadCaster(e)
	b.Store = store

	uid := uuid.NewV4().String()
	b.orders["test"] = &order.Order{
		OType:      0,
		ExecType:   0,
		Code:       "code",
		UUID:       uid,
		Size:       10,
		Price:      21,
		CreatedAt:  time.Time{},
		ExecutedAt: time.Time{},
	}

	e.EXPECT().BroadCast(gomock.AssignableToTypeOf(b.orders["test"]))
	b.Accept("test")

	assert.Len(t, b.positions["code"], 1)
	assert.Equal(t, b.positions["code"][0].Code, "code")
	assert.Equal(t, b.positions["code"][0].Price, float64(21))
	assert.Equal(t, b.positions["code"][0].Size, int64(10))
	assert.Equal(t, order.Completed, b.orders["test"].Status())
}

func TestBroker_Cancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockBroadcaster(ctrl)
	store := mock_store.NewMockStore(ctrl)

	b := NewBroker()
	b.SetEventBroadCaster(e)
	b.Store = store
	input := &order.Order{
		OType:    0,
		ExecType: 0,
		Code:     "code",
		UUID:     "test",
		Size:     21,
		Price:    10,
		StoreUID: "",
	}
	e.EXPECT().BroadCast(input)
	b.orders["test"] = input

	b.Cancel("test")
	assert.Equal(t, order.Canceled, input.Status())
}

func TestBroker_GetCash(t *testing.T) {
	ctrl := gomock.NewController(t)
	b := NewBroker()
	store := mock_store.NewMockStore(ctrl)
	b.Store = store

	store.EXPECT().Cash().Return(int64(100))

	assert.Equal(t, int64(100), b.GetCash())
}

func TestBroker_GetPosition(t *testing.T) {
	ctrl := gomock.NewController(t)
	b := NewBroker()
	store := mock_store.NewMockStore(ctrl)
	b.Store = store

	b.positions["code"] = append(b.positions["code"], position.Position{
		Code:      "code",
		Size:      1,
		Price:     1,
		CreatedAt: time.Time{},
	})

	store.EXPECT().Positions()

	p := b.GetPosition("code")
	assert.Len(t, p, 1)
}

func TestBroker_SetCash(t *testing.T) {
	b := NewBroker()

	b.SetCash(20)
	assert.Equal(t, int64(20), b.Cash)
}
