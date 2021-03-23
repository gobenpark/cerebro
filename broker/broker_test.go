package broker

import (
	"testing"

	mock_event "github.com/gobenpark/trader/event/mock"
	"github.com/gobenpark/trader/order"
	"github.com/golang/mock/gomock"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker(1, 1)
	require.NotNil(t, b)
}

func TestDefaultBroker_Buy(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockEventBroadcaster(ctrl)
	b := NewBroker(1, 0.0005)
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
	result := b.Buy("testcode", 1, 1)
	assert.NotNil(t, result)
}

func TestDefaultBroker_Sell(t *testing.T) {
	ctrl := gomock.NewController(t)
	e := mock_event.NewMockEventBroadcaster(ctrl)
	b := NewBroker(1, 0.0005)
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
	result := b.Sell("testcode", 1, 1)
	assert.NotNil(t, result)

}
