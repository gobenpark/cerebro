package cerebro

type CerebroError struct {
	Code    int
	Message string
}

func (c CerebroError) Error() string {
	return c.Message
}

var (
	ErrUnexpected = CerebroError{Code: 1, Message: "raise unexpected error"}

	ErrStoreNotExists = CerebroError{Code: 2, Message: "store not in cerebro"}
)
