package item

import "fmt"

type StockType int

const (
	KOSPI = iota + 1
	KOSDAQ
)

type Item struct {
	Code     string                 `json:"code"`
	Name     string                 `json:"name"`
	Type     StockType              `json:"type"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (i Item) String() string {
	return fmt.Sprintf("[%s,%s]", i.Code, i.Name)
}
