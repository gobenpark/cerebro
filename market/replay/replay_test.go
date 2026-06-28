package replay

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
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

// TestReplay_CancelRemovesPendingOrder verifies a canceled resting order no longer
// fills when its price condition is later met.
func TestReplay_CancelRemovesPendingOrder(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), o))
	must.NoError(r.Cancel(context.Background(), o))

	is.Empty(r.matchAndFill("AAA", dec(100)), "a canceled order must not fill")
	is.Empty(r.AccountPositions(context.Background()), "no position from a canceled order")
}

// TestReplay_CancelEmitsCanceledEvent verifies Cancel emits a Canceled event on the
// running Events channel, so the broker releases the reservation the live way. ZZZ is
// loaded but never subscribed, so its emitter blocks and the run loop (and its event
// channel) stays alive for the cancellation to emit on.
func TestReplay_CancelEmitsCanceledEvent(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)), WithCandles("ZZZ", indicator.Candles{{Code: "ZZZ", Close: dec(1)}}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := r.Events(ctx)

	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(ctx, o))
	must.NoError(r.Cancel(ctx, o))

	select {
	case e := <-ch:
		ev, ok := e.(market.ChangeOrderEvent)
		must.True(ok, "expected a ChangeOrderEvent")
		is.Equal(o.ID(), ev.ID)
		is.Equal(order.Canceled, ev.Action)
	case <-time.After(time.Second):
		t.Fatal("Cancel did not emit a Canceled event")
	}
}

// TestReplay_CancelBeforeEventsIsBuffered guards that a cancel issued before Events
// is opened (a submit+cancel during Start) is not lost: its Canceled confirmation is
// buffered and flushed once the event stream starts, so the broker can release the
// reservation.
func TestReplay_CancelBeforeEventsIsBuffered(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	r := New(WithBalance(dec(100_000)))
	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(10), dec(100))
	must.NoError(r.Order(context.Background(), o))
	must.NoError(r.Cancel(context.Background(), o)) // before Events — must be buffered

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := r.Events(ctx) // opening the stream flushes the buffered confirmation

	select {
	case e := <-ch:
		ev, ok := e.(market.ChangeOrderEvent)
		must.True(ok, "expected a ChangeOrderEvent")
		is.Equal(o.ID(), ev.ID)
		is.Equal(order.Canceled, ev.Action)
	case <-time.After(time.Second):
		t.Fatal("a cancel before Events was not flushed when the stream opened")
	}
}

// TestReplay_ShutdownNotBlockedByStuckCancelSend guards that a cancel send blocked on
// a full, undrained event channel (with a non-canceling context) does not deadlock
// the replay's shutdown: closeEvents releases the blocked send via stopSend, so the
// run loop can still close the stream when its context is canceled.
func TestReplay_ShutdownNotBlockedByStuckCancelSend(t *testing.T) {
	// ZZZ is loaded but never subscribed, so the run loop stays alive until runCtx is
	// canceled; the event channel is intentionally never drained.
	r := New(WithBalance(dec(100_000)), WithCandles("ZZZ", indicator.Candles{{Code: "ZZZ", Close: dec(1)}}))
	runCtx, runCancel := context.WithCancel(context.Background())
	_ = r.Events(runCtx)

	// Flood cancels with a non-canceling context: once the 16-slot buffer fills, a
	// send blocks. It must not hold emitMu, or the shutdown below would deadlock.
	go func() {
		for range 30 {
			o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, dec(1), dec(100))
			_ = r.Order(context.Background(), o)
			_ = r.Cancel(context.Background(), o)
		}
	}()
	time.Sleep(50 * time.Millisecond) // let the buffer fill and a send block

	runCancel()
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("replay shutdown deadlocked on a blocked cancel send")
	}
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

// TestReplay_CandlesReturnsWarmupNotReplayWindow guards against warm-up look-ahead:
// Candles must return the separate WithWarmup history, never the WithCandles window
// that is streamed as ticks (which would leak future bars and duplicate them live).
func TestReplay_CandlesReturnsWarmupNotReplayWindow(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	window := indicator.Candles{{Code: "AAA", Close: dec(100)}, {Code: "AAA", Close: dec(101)}}
	warmMin := indicator.Candles{{Code: "AAA", Close: dec(98)}}
	warmMin5 := indicator.Candles{{Code: "AAA", Close: dec(97)}, {Code: "AAA", Close: dec(96)}}
	r := New(
		WithCandles("AAA", window),
		WithWarmup("AAA", market.Min, warmMin),
		WithWarmup("AAA", market.Min5, warmMin5),
	)

	got, err := r.Candles(context.Background(), "AAA", market.Min)
	must.NoError(err)
	must.Len(got, 1, "Candles must return the warm-up set, not the 2-bar replay window")
	is.True(got[0].Close.Equal(dec(98)))

	// Same code, different level: must return that level's own warm-up, not the other's.
	got5, err := r.Candles(context.Background(), "AAA", market.Min5)
	must.NoError(err)
	must.Len(got5, 2)
	is.True(got5[0].Close.Equal(dec(97)))

	// A code with a replay window but no warm-up cold-starts: Candles returns nil.
	none, err := r.Candles(context.Background(), "BBB", market.Min)
	must.NoError(err)
	is.Nil(none)
}
