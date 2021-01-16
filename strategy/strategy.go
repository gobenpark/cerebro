package strategy

type Strategy interface {
	Next()

	NotifyOrder()
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
