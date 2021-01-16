package cerebro

import (
	mock_broker "github.com/gobenpark/trader/broker/mock"
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
