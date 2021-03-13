package domain

type Container interface {
	Empty() bool
	Size() int
	Clear()
	Values() []interface{}
	Add(data interface{}) error
}
