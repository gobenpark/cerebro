package risk_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/risk"
)

// recorder is a fake Submitter that captures exit orders and, like a scoped
// broker, stamps the strategy onto each one.
type recorder struct {
	strategy string
	orders   []order.Order
}

func (r *recorder) Order(_ context.Context, o order.Order, _ bool) error {
	o.SetStrategy(r.strategy)
	r.orders = append(r.orders, o)
	return nil
}

// fakeBook stands in for the broker's per-strategy position ledger.
type fakeBook struct {
	pos  map[string]map[string]position.Position
	peak map[string]map[string]decimal.Decimal
}

func newFakeBook() *fakeBook {
	return &fakeBook{
		pos:  map[string]map[string]position.Position{},
		peak: map[string]map[string]decimal.Decimal{},
	}
}

func (f *fakeBook) set(strategy, code string, size, avg int64) {
	if f.pos[strategy] == nil {
		f.pos[strategy] = map[string]position.Position{}
	}
	f.pos[strategy][code] = position.Position{Item: &item.Item{Code: code}, Size: dec(size), Price: dec(avg)}
}

// setPeak overrides the reported high-water fill price (defaults to the average).
func (f *fakeBook) setPeak(strategy, code string, peak int64) {
	if f.peak[strategy] == nil {
		f.peak[strategy] = map[string]decimal.Decimal{}
	}
	f.peak[strategy][code] = dec(peak)
}

func (f *fakeBook) clear(strategy, code string) {
	if f.pos[strategy] != nil {
		delete(f.pos[strategy], code)
	}
	if f.peak[strategy] != nil {
		delete(f.peak[strategy], code)
	}
}

func (f *fakeBook) StrategyPosition(strategy, code string) (position.Position, decimal.Decimal, bool) {
	p, ok := f.pos[strategy][code]
	if !ok {
		return position.Position{}, decimal.Zero, false
	}
	peak, has := f.peak[strategy][code]
	if !has {
		peak = p.Price // default the high-water mark to the average entry
	}
	return p, peak, true
}

func newMonitor(t *testing.T, rec *recorder, book risk.Book, p risk.Policy) *risk.Monitor {
	t.Helper()
	lg := slog.New(slog.DiscardHandler)
	return risk.NewMonitor(lg, map[string]risk.Policy{rec.strategy: p},
		func(string) risk.Submitter { return rec }, book)
}

func tick(code string, price int64) indicator.Tick {
	return indicator.Tick{Code: code, Price: dec(price)}
}

func TestMonitor_StopLossExits(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{StopLoss: 0.05}) // exit at/below 95
	ctx := context.Background()

	m.Listen(ctx, tick("AAA", 96))
	is.Empty(rec.orders, "above the stop: no exit")

	m.Listen(ctx, tick("AAA", 95))
	must.Len(rec.orders, 1, "stop breached: one exit")
	is.Equal(order.Sell, rec.orders[0].Action())
	is.Equal(order.Market, rec.orders[0].Type())
	is.True(dec(10).Equal(rec.orders[0].Size()), "exits the whole position")

	m.Listen(ctx, tick("AAA", 90))
	is.Len(rec.orders, 1, "in-flight guard prevents a double exit while the position remains")
}

func TestMonitor_TrailingStopExits(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{TrailingStop: 0.10}) // 10% off the peak
	ctx := context.Background()

	m.Listen(ctx, tick("AAA", 120)) // peak -> 120, stop -> 108
	m.Listen(ctx, tick("AAA", 109))
	is.Empty(rec.orders, "above the trailing stop")

	m.Listen(ctx, tick("AAA", 108))
	is.Len(rec.orders, 1, "trailing stop breached")
}

func TestMonitor_TakeProfitExits(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{TakeProfit: 0.10}) // exit at/above 110
	ctx := context.Background()

	m.Listen(ctx, tick("AAA", 109))
	is.Empty(rec.orders)

	m.Listen(ctx, tick("AAA", 110))
	is.Len(rec.orders, 1)
}

func TestMonitor_IgnoresStrategyWithoutPosition(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook() // dip holds nothing
	m := newMonitor(t, rec, book, risk.Policy{StopLoss: 0.05})
	ctx := context.Background()

	m.Listen(ctx, tick("AAA", 50))
	is.Empty(rec.orders, "no position, nothing to exit")
}

func TestMonitor_FlatPositionReArms(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{StopLoss: 0.05})
	ctx := context.Background()

	m.Listen(ctx, tick("AAA", 95))
	must.Len(rec.orders, 1)

	// The exit fills: the position goes flat, which clears the guard on the next tick.
	book.clear("dip", "AAA")
	m.Listen(ctx, tick("AAA", 100))

	// A fresh entry is monitored again.
	book.set("dip", "AAA", 10, 100)
	m.Listen(ctx, tick("AAA", 95))
	is.Len(rec.orders, 2, "a re-entered position is monitored again")
}

func TestMonitor_TrailingPeakIncludesFillHigh(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 20, 150) // scaled in at 100 and 200: average 150
	book.setPeak("dip", "AAA", 200) // high-water fill price is 200
	m := newMonitor(t, rec, book, risk.Policy{TrailingStop: 0.10})
	ctx := context.Background()

	// First tick at 175: against the 200 fill high the 10% stop is 180, so 175 exits —
	// even though the Monitor has not itself observed a tick above 175.
	m.Listen(ctx, tick("AAA", 175))
	is.Len(rec.orders, 1, "trailing peak must include the fill high (200), not just the average entry")
}

func TestMonitor_TrailingPeakResetsOnExitFill(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{TrailingStop: 0.10}) // 10% off the peak
	ctx := context.Background()

	// Build the peak up to 120, then breach the trailing stop at 108.
	m.Listen(ctx, tick("AAA", 120))
	m.Listen(ctx, tick("AAA", 108))
	must.Len(rec.orders, 1, "trailing stop fires")

	// The exit fills (full close) and the strategy re-enters at 100 with NO tick
	// observing the flat state in between.
	exit := rec.orders[0]
	exit.Complete()
	book.clear("dip", "AAA")
	m.Listen(ctx, exit) // must reset the stale peak (120), not just the guard
	book.set("dip", "AAA", 10, 100)

	// A tick at 108 must NOT exit: the fresh position's peak is 108 (stop ~97), not
	// the prior trade's 120 (stop 108).
	m.Listen(ctx, tick("AAA", 108))
	is.Len(rec.orders, 1, "a fresh position must not inherit the prior trade's high-water mark")
}

func TestMonitor_PeakResetsOnManualClose(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{TrailingStop: 0.10})
	ctx := context.Background()

	// Build the peak up to 130 (stop would be 117, not yet breached).
	m.Listen(ctx, tick("AAA", 130))
	must.Empty(rec.orders)

	// The strategy closes the position itself (its own completed sell), the book goes
	// flat, and it re-enters at 100 before any tick observes the flat state.
	sell := order.NewOrder(&item.Item{Code: "AAA"}, order.Sell, order.Limit, dec(10), dec(130))
	sell.SetStrategy("dip")
	sell.Complete()
	book.clear("dip", "AAA")
	m.Listen(ctx, sell) // not the Monitor's exit, but the book is now flat
	book.set("dip", "AAA", 10, 100)

	// A tick at 117 must NOT exit: the fresh position's peak is 117 (stop ~105), not
	// the prior trade's 130 (stop 117).
	m.Listen(ctx, tick("AAA", 117))
	is.Empty(rec.orders, "a fresh position must not inherit the prior trade's peak after a manual close")
}

func TestMonitor_RejectedExitReArms(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	book := newFakeBook()
	book.set("dip", "AAA", 10, 100)
	m := newMonitor(t, rec, book, risk.Policy{StopLoss: 0.05})
	ctx := context.Background()

	m.Listen(ctx, tick("AAA", 95))
	must.Len(rec.orders, 1)

	// The exit is rejected (the position never closed): the terminal order frees the
	// guard so a subsequent breach retries.
	exit := rec.orders[0]
	exit.Reject()
	m.Listen(ctx, exit)

	m.Listen(ctx, tick("AAA", 94))
	is.Len(rec.orders, 2, "a rejected exit is retried on the next breach")
}
