package market

//go:generate mockgen -source=./market.go -destination=./mock/mock_market.go

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

type (
	CandleType int

	// TickEventHandler returns the items whose realtime tick feed should be
	// subscribed. It is passed to Subscribe.
	TickEventHandler func() []*item.Item
)

const (
	Min CandleType = iota + 1
	Min2
	Min3
	Min4
	Min5
	Day
	Week
)

// Duration is the wall-clock width of one candle at this level. It maps a level to
// the compress a Resampler folds live ticks with, so a warm-up's historical level
// and the live bars it continues stay the same width. An unknown level returns 0.
func (c CandleType) Duration() time.Duration {
	switch c {
	case Min:
		return time.Minute
	case Min2:
		return 2 * time.Minute
	case Min3:
		return 3 * time.Minute
	case Min4:
		return 4 * time.Minute
	case Min5:
		return 5 * time.Minute
	case Day:
		return 24 * time.Hour
	case Week:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

type Market interface {
	Stocks(ctx context.Context) []*item.Item
	Candles(ctx context.Context, code string, level CandleType) (indicator.Candles, error)
	Subscribe(ctx context.Context, handler TickEventHandler) error
	Order(ctx context.Context, o order.Order) error
	// Cancel requests cancellation of a resting order. Like Order it is asynchronous:
	// the adapter sends the request and the broker releases the order's reservation
	// when the exchange confirms with a ChangeOrderEvent of status Canceled. Canceling
	// an order that is unknown or already terminal is a no-op (nil error).
	Cancel(ctx context.Context, o order.Order) error
	AccountPositions(ctx context.Context) []position.Position
	AccountBalance(ctx context.Context) decimal.Decimal
	// Events streams the adapter's ticks, order/balance changes, and optionally
	// order-book snapshots (indicator.OrderBook, delivered to a strategy's
	// Universe.OrderBooks) until ctx is canceled. Liveness contract for a live
	// adapter: it must survive a transient
	// disconnect by reconnecting internally and keeping this channel open — a drop
	// must not close the channel. It SHOULD emit a FeedStatusEvent on disconnect and
	// reconnect so operators see feed health and Cerebro's staleness watchdog
	// (cerebro.WithFeedTimeout) is reset during quiet, tickless periods. Closing the
	// channel signals a permanent end of the feed; with the watchdog armed Cerebro
	// treats a close while the run is still live as feed loss. A backtest adapter
	// (e.g. replay) legitimately closes the channel when its data is exhausted, so
	// the watchdog is meant for live feeds, not backtests.
	Events(ctx context.Context) <-chan any
	// Commission is the fee rate applied to an order's value, as a Rate whose unit
	// (percentage vs fraction) is fixed by its constructor — see market.Percent /
	// market.Fraction. It must be a cheap, non-blocking accessor (a cached/constant
	// value): it is read inside the broker's lock and on every fill, so it takes no
	// context and must not do I/O.
	Commission() Rate
}

// Unsubscriber is an optional capability a Market adapter may implement so Cerebro
// can release a set of codes' feed when a dynamic watchlist (a screener) drops them.
// An adapter that does not implement it keeps streaming those codes; Cerebro then
// simply stops routing their updates to any strategy. Like Subscribe, it may be
// called more than once over a run as the watchlist churns.
type Unsubscriber interface {
	Unsubscribe(ctx context.Context, codes []string) error
}
