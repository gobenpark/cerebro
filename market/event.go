package market

import (
	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/order"
)

type MarketEvent interface {
	String() string
}

type ChangeOrderEvent struct {
	Message string
	ID      string
	Action  order.Status
	// FilledSize is the quantity filled by this event. It is applied to the
	// order's remaining size on a Partial fill; other actions ignore it.
	FilledSize decimal.Decimal
	// Price is the price this event filled at, used for PnL accounting. Zero means
	// the adapter did not report it; the broker then falls back to the order's own
	// (limit) price, which is exact for limit orders but unknown for market orders.
	Price decimal.Decimal
}

func (o ChangeOrderEvent) String() string {
	return o.Message
}

type ChangeBalanceEvent struct {
	Message string
	Balance decimal.Decimal
}

func (o ChangeBalanceEvent) String() string {
	return o.Message
}
