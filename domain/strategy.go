package domain

type Strategy interface {
	Next(broker Broker, container Container)

	NotifyOrder()
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
