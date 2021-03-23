package domain

type OrderType int

const (
	Buy OrderType = iota + 1
	Sell
)
