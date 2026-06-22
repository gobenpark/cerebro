package order_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

// TestOrder_CopyPreservesStrategy guards that Copy carries the strategy tag, so
// the order-update copies the broker broadcasts on fills keep their attribution.
func TestOrder_CopyPreservesStrategy(t *testing.T) {
	is := assert.New(t)

	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, decimal.NewFromInt(1), decimal.NewFromInt(100))
	o.SetStrategy("alpha")

	is.Equal("alpha", o.Copy().Strategy())
}
