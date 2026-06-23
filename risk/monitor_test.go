package risk_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	logv1 "github.com/gobenpark/cerebro/log/v1"
	"github.com/gobenpark/cerebro/order"
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

func newMonitor(t *testing.T, rec *recorder, p risk.Policy) *risk.Monitor {
	t.Helper()
	lg, err := logv1.NewLogger(log.ErrorLevel)
	require.NoError(t, err)
	return risk.NewMonitor(lg, map[string]risk.Policy{rec.strategy: p},
		func(string) risk.Submitter { return rec })
}

func filledBuy(strategy, code string, size, price int64) order.Order {
	o := order.NewOrder(&item.Item{Code: code}, order.Buy, order.Limit, dec(size), dec(price))
	o.SetStrategy(strategy)
	o.Complete()
	return o
}

func tick(code string, price int64) indicator.Tick {
	return indicator.Tick{Code: code, Price: dec(price)}
}

func TestMonitor_StopLossExits(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	m := newMonitor(t, rec, risk.Policy{StopLoss: 0.05}) // exit at/below 95
	ctx := context.Background()

	m.Listen(ctx, filledBuy("dip", "AAA", 10, 100))
	m.Listen(ctx, tick("AAA", 96))
	is.Empty(rec.orders, "above the stop: no exit")

	m.Listen(ctx, tick("AAA", 95))
	must.Len(rec.orders, 1, "stop breached: one exit")
	is.Equal(order.Sell, rec.orders[0].Action())
	is.Equal(order.Market, rec.orders[0].Type())
	is.True(dec(10).Equal(rec.orders[0].Size()), "exits the whole position")

	m.Listen(ctx, tick("AAA", 90))
	is.Len(rec.orders, 1, "in-flight guard prevents a double exit")
}

func TestMonitor_TrailingStopExits(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	m := newMonitor(t, rec, risk.Policy{TrailingStop: 0.10}) // 10% off the peak
	ctx := context.Background()

	m.Listen(ctx, filledBuy("dip", "AAA", 10, 100))
	m.Listen(ctx, tick("AAA", 120)) // peak -> 120, stop -> 108
	m.Listen(ctx, tick("AAA", 109))
	is.Empty(rec.orders, "above the trailing stop")

	m.Listen(ctx, tick("AAA", 108))
	is.Len(rec.orders, 1, "trailing stop breached")
}

func TestMonitor_TakeProfitExits(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	m := newMonitor(t, rec, risk.Policy{TakeProfit: 0.10}) // exit at/above 110
	ctx := context.Background()

	m.Listen(ctx, filledBuy("dip", "AAA", 10, 100))
	m.Listen(ctx, tick("AAA", 109))
	is.Empty(rec.orders)

	m.Listen(ctx, tick("AAA", 110))
	is.Len(rec.orders, 1)
}

func TestMonitor_IgnoresUnattributedAndOtherStrategies(t *testing.T) {
	is := assert.New(t)
	rec := &recorder{strategy: "dip"}
	m := newMonitor(t, rec, risk.Policy{StopLoss: 0.05})
	ctx := context.Background()

	// A fill owned by another strategy is not tracked by this policy.
	m.Listen(ctx, filledBuy("other", "AAA", 10, 100))
	m.Listen(ctx, tick("AAA", 90))
	is.Empty(rec.orders, "only the policy's own strategy is monitored")
}

func TestMonitor_ExitClearsAndReArms(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	m := newMonitor(t, rec, risk.Policy{StopLoss: 0.05})
	ctx := context.Background()

	m.Listen(ctx, filledBuy("dip", "AAA", 10, 100))
	m.Listen(ctx, tick("AAA", 95))
	must.Len(rec.orders, 1)

	// The exit fills: feed it back as a completed sell (its price is 0, a market
	// order, so the monitor values it at the last tick price).
	exit := rec.orders[0]
	exit.Complete()
	m.Listen(ctx, exit)

	// Position is flat again; a fresh entry re-arms the policy.
	m.Listen(ctx, filledBuy("dip", "AAA", 10, 100))
	m.Listen(ctx, tick("AAA", 95))
	is.Len(rec.orders, 2, "a re-entered position is monitored again")
}

func TestMonitor_PartialThenCompleteCountsFillOnce(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	rec := &recorder{strategy: "dip"}
	m := newMonitor(t, rec, risk.Policy{StopLoss: 0.05})
	ctx := context.Background()

	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	o.SetStrategy("dip")
	o.Partial(dec(4)) // 4 filled, 6 remaining
	m.Listen(ctx, o)
	o.Complete() // the remaining 6 fills
	m.Listen(ctx, o)

	m.Listen(ctx, tick("AAA", 95))
	must.Len(rec.orders, 1)
	is.True(dec(10).Equal(rec.orders[0].Size()), "the partial and completion sum to 10, not 14")
}
