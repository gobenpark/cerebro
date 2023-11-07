package item

type StockType int

const (
	KOSPI = iota + 1
	KOSDAQ
)

type Item struct {
	Code string            `json:"code"`
	Type StockType         `json:"type"`
	Name string            `json:"name"`
	Tag  map[string]string `json:"other"`
}
