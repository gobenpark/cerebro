package strategy

import (
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/order"
)

type Strategy interface {
	Next(broker broker.Broker, container domain.Container)

	NotifyOrder(o *order.Order)
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
