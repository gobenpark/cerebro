package event

type EventType int

const (
	Order int = iota + 1
)

type Event struct {
	UUID string
}
