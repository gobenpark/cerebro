package broker

import (
	"testing"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker(1, 1)
	require.NotNil(t, b)
}

func TestDefaultBroker_Buy(t *testing.T) {
	b := NewBroker(1000, 0.005)
	ech := make(chan event.Event, 1)
	b.event = ech
	uid := b.Buy("test code", 10, 10)
	require.Len(t, b.orders, 1)

	assert.Equal(t, b.orders[uid].Status, order.Submitted)
	event := <-ech
	t.Log(event.UUID)
}

func TestDefaultBroker_Cancel(t *testing.T) {
	b := NewBroker(1000, 0.005)
	ech := make(chan event.Event, 1)
	b.event = ech
	uid := b.Buy("test", 100, 10)
	assert.Len(t, b.orders, 1)
	t.Log(<-ech)
	b.Cancel(uid)
	assert.Len(t, b.orders, 1)
	assert.Equal(t, b.orders[uid].Status, order.Canceled)
}
