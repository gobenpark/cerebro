package strategy

import (
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
)

type Filter interface {
	Pass(item item.Item, c indicator.Candles) bool
}
