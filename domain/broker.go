package domain

type Broker interface {
	Buy(size int64, price float64)
	Sell(size int64, price float64)
	Cancel(uuid string)
	Submit(uid string)
	GetPosition()
	AddOrderHistory()
	SetFundHistory()
	CommissionInfo()
	SetCash(cash int64)
}
