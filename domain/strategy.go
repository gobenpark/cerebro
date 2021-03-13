package domain

import (
	"context"

	"github.com/gobenpark/trader/event"
)

type Strategy interface {
	Next()

	NotifyOrder()
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
	Start(ctx context.Context, event chan event.Event)
	Buy(code string, size int64, price float64)
	Sell(code string, size int64, price float64)
}
