package broker

import (
	"testing"
	"time"

	"github.com/gobenpark/trader/position"
	mock_store "github.com/gobenpark/trader/store/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func HelperNewBroker(t *testing.T) (Broker, *mock_store.MockStore) {
	t.Helper()

	ctrl := gomock.NewController(t)
	st := mock_store.NewMockStore(ctrl)
	broker := NewBroker(st)
	return broker, st
}

func TestBroker_GetCash(t *testing.T) {
	b, st := HelperNewBroker(t)
	st.EXPECT().Cash().Return(int64(10))

	cash := b.GetCash()
	assert.Equal(t, int64(10), cash)
}

func TestBroker_GetPosition(t *testing.T) {
	b, st := HelperNewBroker(t)

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

func TestBroker_SetStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := mock_store.NewMockStore(ctrl)

	b := broker{
		store: nil,
	}
	b.SetStore(st)

	assert.Equal(t, st, b.store)
}
