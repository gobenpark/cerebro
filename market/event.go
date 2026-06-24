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

// FeedState is the connection state of a market-data feed, reported by an adapter
// through a FeedStatusEvent.
type FeedState int

const (
	// FeedDisconnected means the feed dropped and the adapter is (re)connecting. A
	// contract-compliant adapter keeps its Events channel open across this state.
	FeedDisconnected FeedState = iota
	// FeedConnected means the feed is live and streaming.
	FeedConnected
)

func (s FeedState) String() string {
	switch s {
	case FeedConnected:
		return "connected"
	case FeedDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// FeedStatusEvent lets a market adapter report the health of its data feed. Emitting
// one serves two purposes: it surfaces feed health to operators (Cerebro logs it),
// and it resets Cerebro's staleness watchdog (see cerebro.WithFeedTimeout). The
// latter lets an adapter that reconnects during a quiet period — when no ticks would
// otherwise flow — avoid being mistaken for a dead feed. Emitting it is optional; the
// watchdog also resets on ordinary ticks.
type FeedStatusEvent struct {
	State   FeedState
	Message string
}

func (e FeedStatusEvent) String() string { return e.Message }
