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
	// Order submits an order to the exchange. o.ID() is a stable client order id (an
	// idempotency key): send it to the exchange as the client order id so that a
	// retried submission is de-duplicated rather than doubled, and so the resulting
	// ChangeOrderEvent (matched by ID) and any restart recovery (OpenOrderReporter)
	// line up with the broker's view. The call should honor ctx; if it exceeds
	// cerebro's WithOrderTimeout the broker treats the submission as in-doubt — the
	// order is kept open and its cash reserved, NOT rejected — so a slow ack does not
	// turn a possibly-live order into a phantom.
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

// OpenOrderReporter is an optional capability a live Market adapter implements so
// Cerebro can recover resting orders on restart — the orders the exchange still has
// working that the local process forgot when it stopped. Cerebro seeds them into the
// broker's open set on Start (broker.ReconcileOpenOrders) so their cash is reserved
// again and their later fill/cancel events are recognized and applied, rather than
// arriving as unknown orders.
//
// Contract for the returned orders:
//   - each must carry the exchange's order id (order.SetID) so subsequent
//     ChangeOrderEvents match it;
//   - Size() must be the order's ORIGINAL quantity and RemainingSize() its still-open
//     remainder — construct the order with the original size, then apply the already-
//     filled portion (e.g. order.Partial), do NOT build it straight from the remainder.
//     RemainingSize drives the cash reservation; the broker books a completion as Size()
//     minus the fills it has observed (persisted across restarts), so reporting Size() as
//     the remainder would under-book the completion (the broker drops obviously
//     inconsistent progress and warns, but cannot recover the original size);
//   - attribution: the exchange does not record which strategy placed an order. The
//     adapter SHOULD recover it from the client order id (order.SetStrategy) — that is
//     the reliable source. Absent that, the broker attributes only an unattributed SELL
//     to the sole strategy holding a lot in its code (so a recovered exit closes the
//     right position); a buy, or a code held by several strategies, stays unattributed,
//     so an adapter that opens positions across restarts or runs several strategies in
//     one code must supply the strategy from the client id.
//
// A backtest adapter (e.g. replay) need not implement this — reconciliation is then a
// no-op, which is the correct behavior for a cold-started simulation.
//
// Startup consistency: Cerebro takes this snapshot during Start and only begins
// consuming Events afterward, so the adapter MUST NOT lose order/balance events that
// occur after the snapshot is taken — it should already be connected and buffer them
// (the same persistent-connection behavior the Events liveness contract assumes), so
// they are delivered once Cerebro drains the stream. An adapter that subscribes to
// order updates only inside Events, discarding anything earlier, can drop a recovered
// order's fill/cancel in that window; the order then stays open with its cash reserved
// until a later reconcile or restart. That failure is conservative (it over-reserves,
// never under-reserves, so it cannot cause a double-commit), but a faithful live
// adapter avoids it by buffering from connection. Fully closing the snapshot-vs-stream
// gap without that guarantee needs exchange sequence numbers (replay events past the
// snapshot's sequence) and is out of scope here.
type OpenOrderReporter interface {
	OpenOrders(ctx context.Context) ([]order.Order, error)
}
