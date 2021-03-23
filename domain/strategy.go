package domain

import "github.com/gobenpark/trader/order"

type Strategy interface {
	Next(broker Broker, container Container)

	NotifyOrder(o *order.Order)
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
