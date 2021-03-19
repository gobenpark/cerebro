//go:generate mockgen -source=./container.go -destination=./mock/mock_container.go
package domain

type Container interface {
	Empty() bool
	Size() int
	Clear()
	Values() []Candle
	Add(candle Candle)
	Code() string
}
