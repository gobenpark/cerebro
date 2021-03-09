package domain

import "github.com/gobenpark/trader/position"

type Broker interface {
	Buy(code string, size int64, price float64) string
	Sell(code string, size int64, price float64) string
	Cancel(uuid string)
	Submit(uid string)
	GetPosition(code string) (position.Position, error)
	AddOrderHistory()
	SetFundHistory()
	CommissionInfo()
	SetCash(cash int64)
}
