package cerebro

type CerebroError struct {
	Code    int
	Message string
}

var (
	ErrUnexpected = CerebroError{Code: 1, Message: "raise unexpected error"}
)
