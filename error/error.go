package error

type Error struct {
	Code    int
	Message string
}

func (c Error) Error() string {
	return c.Message
}

var (
	ErrUnexpected     = Error{Code: 1, Message: "raise unexpected error"}
	ErrStoreNotExists = Error{Code: 2, Message: "store not in cerebro"}
	ErrNotExistCode   = Error{Code: 3, Message: "does not exist code"}

	ErrNotEnoughMoney = Error{Code: 4, Message: "not enough money"}
)
