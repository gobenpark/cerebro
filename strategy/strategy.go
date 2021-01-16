package strategy

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

type Strategy interface {
	Next()

	NotifyOrder()
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
