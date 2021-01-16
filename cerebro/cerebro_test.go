package cerebro

import (
	mock_broker "github.com/gobenpark/trader/broker/mock"
	mock_store "github.com/gobenpark/trader/store/mock"
	mock_strategy "github.com/gobenpark/trader/strategy/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewCerebro(t *testing.T) {
	ctrl := gomock.NewController(t)
	broker := mock_broker.NewMockBroker(ctrl)
	cerebro := NewCerebro(broker)
	require.NotNil(t, cerebro)
}

func Test_cerebro_AddStrategy(t *testing.T) {
	ctrl := gomock.NewController(t)
	strategy := mock_strategy.NewMockStrategy(ctrl)
	cerebro := cerebro{}
	cerebro.AddStrategy(strategy)
	require.Len(t, cerebro.Strategies, 1)
}

func Test_cerebro_AddStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mock_store.NewMockStorer(ctrl)
	cerebro := cerebro{}
	cerebro.AddStore(store)
	require.Len(t, cerebro.Stores, 1)
}
