package replay

import (
	"context"
	"testing"
	"time"

	"github.com/gobenpark/cerebro/market"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

func dec(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

func TestReplay_LimitBuyFillsAtLimitAndDebitsCash(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)), WithCommission(market.Fraction(decimal.Zero)))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), o))

	// price 100 <= limit 100 -> fills at 100, cost 1000.
	events := r.matchAndFill("AAA", dec(100))

	must.Len(events, 2)
	is.True(dec(99_000).Equal(r.AccountBalance(context.Background())))
	pos := r.AccountPositions(context.Background())
	must.Len(pos, 1)
	is.Equal("AAA", pos[0].Item.Code)
	is.True(dec(10).Equal(pos[0].Size))
	is.True(dec(100).Equal(pos[0].Price))
	is.Equal(order.Completed, o.Status())
}

func TestReplay_LimitBuyStaysOpenAboveLimit(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), o))

	// price 101 > limit 100 -> no fill, order stays open.
	events := r.matchAndFill("AAA", dec(101))

	is.Empty(events)
	is.True(dec(100_000).Equal(r.AccountBalance(context.Background())))
	is.Empty(r.AccountPositions(context.Background()))
}

func TestReplay_CommissionDebitedOnFill(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	// 1% commission on 1000 notional -> 10 fee.
	r := New(WithBalance(dec(100_000)), WithCommission(market.Percent(decimal.NewFromInt(1))))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), o))

	r.matchAndFill("AAA", dec(100))

	is.True(dec(98_990).Equal(r.AccountBalance(context.Background())), "100000 - 1000 notional - 10 fee")
}

func TestReplay_LimitSellAddsCashAndReducesPosition(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	buy := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), buy))
	r.matchAndFill("AAA", dec(100)) // -1000, +10 @ 100

	sell := order.NewOrder(&item.Item{Code: "AAA"}, order.Sell, order.Limit, dec(4), dec(120))
	must.NoError(r.Order(context.Background(), sell))
	// price 120 >= limit 120 -> sell 4 @ 120 = +480.
	r.matchAndFill("AAA", dec(120))

	is.True(dec(99_480).Equal(r.AccountBalance(context.Background())), "99000 + 480")
	pos := r.AccountPositions(context.Background())
	must.Len(pos, 1)
	is.True(dec(6).Equal(pos[0].Size), "10 - 4")
}

func TestReplay_MarketOrderFillsAtCurrentPrice(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Market, dec(5), dec(0))
	must.NoError(r.Order(context.Background(), o))

	r.matchAndFill("AAA", dec(200)) // market buy fills at the current price 200

	is.True(dec(99_000).Equal(r.AccountBalance(context.Background())), "100000 - 5*200")
	pos := r.AccountPositions(context.Background())
	must.Len(pos, 1)
	is.True(dec(200).Equal(pos[0].Price))
}

func TestReplay_SellWithoutHoldingsDoesNotFill(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	sell := order.NewOrder(&item.Item{Code: "AAA"}, order.Sell, order.Limit, dec(5), dec(100))
	must.NoError(r.Order(context.Background(), sell))

	// price 120 >= limit 100 satisfies the limit, but there is no position to sell.
	events := r.matchAndFill("AAA", dec(120))

	is.Empty(events, "a sell with no holdings must not fill")
	is.True(dec(100_000).Equal(r.AccountBalance(context.Background())), "no cash may be minted")
	is.Empty(r.AccountPositions(context.Background()))
}

func TestReplay_OversellDoesNotFill(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	buy := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(3), dec(100))
	must.NoError(r.Order(context.Background(), buy))
	r.matchAndFill("AAA", dec(100)) // hold 3

	sell := order.NewOrder(&item.Item{Code: "AAA"}, order.Sell, order.Limit, dec(5), dec(100))
	must.NoError(r.Order(context.Background(), sell))
	events := r.matchAndFill("AAA", dec(120))

	is.Empty(events, "cannot oversell 5 while holding 3")
	pos := r.AccountPositions(context.Background())
	must.Len(pos, 1)
	is.True(dec(3).Equal(pos[0].Size), "position is untouched")
}

// TestReplay_UntargetedCodeNeitherEmitsNorHangs guards against a hang when the
// loaded data contains a code that the strategy never subscribes to: that code
// must simply not emit, while subscribed codes still replay normally.
func TestReplay_UntargetedCodeNeitherEmitsNorHangs(t *testing.T) {
	is := assert.New(t)

	r := New(
		WithCandles("AAA", indicator.Candles{{Code: "AAA", Close: dec(100)}}),
		WithCandles("BBB", indicator.Candles{{Code: "BBB", Close: dec(100)}}),
	)
	// Only AAA is subscribed; BBB has data but is never targeted.
	_ = r.Subscribe(context.Background(), func() []*item.Item { return []*item.Item{{Code: "AAA"}} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := r.Events(ctx)

	var codes []string
	timeout := time.After(time.Second)
	for done := false; !done; {
		select {
		case e, ok := <-ch:
			if !ok {
				done = true
				break
			}
			if tk, isTick := e.(indicator.Tick); isTick {
				codes = append(codes, tk.Code)
			}
		case <-timeout:
			done = true
		}
	}

	is.Contains(codes, "AAA", "subscribed code must emit")
	is.NotContains(codes, "BBB", "unsubscribed code must not emit (and must not hang the run)")
}

func TestReplay_DoneClosesWhenReplayFinishes(t *testing.T) {
	r := New(WithCandles("AAA", indicator.Candles{{Code: "AAA", Close: dec(100)}}))
	_ = r.Subscribe(context.Background(), func() []*item.Item { return []*item.Item{{Code: "AAA"}} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := r.Events(ctx)
	go func() { //nolint:revive // drain so the emitter can run to completion
		for range ch {
		}
	}()

	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("Done did not close after the replay finished")
	}
}

func TestReplay_OtherCodeIsNotFilled(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), o))

	is.Empty(r.matchAndFill("BBB", dec(50)), "a tick for another code must not fill this order")
	is.True(dec(100_000).Equal(r.AccountBalance(context.Background())))
}
