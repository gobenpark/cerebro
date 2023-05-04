package event

type EventError struct {
	Message string
}

func (e EventError) Error() string {
	return e.Message
}
