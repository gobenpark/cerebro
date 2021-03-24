package strategy

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

import (
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
)

type Strategy interface {
	Next(broker broker.Broker, container container.Container)

	NotifyOrder(o *order.Order)
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
