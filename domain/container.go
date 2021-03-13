package domain

type Container interface {
	Empty() bool
	Size() int
	Clear()
	Values(code string) []Candle
	Add(candle Candle)
}
