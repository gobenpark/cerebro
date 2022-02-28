package event

type ErrorEvent struct {
	Message string
}

func (e ErrorEvent) Error() string {
	return e.Message
}
