package broker

import (
	"context"
	"testing"

	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	mock_store "github.com/gobenpark/trader/store/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestBrokerSuite struct {
	suite.Suite
	*Broker
}

func (suite *TestBrokerSuite) SetupTest() {
	ctrl := gomock.NewController(suite.T())
	mst := mock_store.NewMockStore(ctrl)

	suite.Broker = NewBroker(event.NewEventEngine(), mst, 0.03, 10000, nil)
}

func (suite *TestBrokerSuite) TestOrderFail() {
	assert.Error(suite.T(), suite.Broker.Order(context.TODO(), "testcode", 10, 1000, order.Buy, order.Limit))
}

func (suite *TestBrokerSuite) TestOrderSuccess() {
	assert.NoError(suite.T(), suite.Broker.Order(context.TODO(), "testcode", 9, 1000, order.Buy, order.Limit))
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(TestBrokerSuite))
}
