package item

type StockType int

const (
	KOSPI = iota + 1
	KOSDAQ
)

type Item struct {
	Tag  map[string]string `json:"other"`
	Code string            `json:"code"`
	Name string            `json:"name"`
	Type StockType         `json:"type"`
}
