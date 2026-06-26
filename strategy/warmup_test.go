/*
 *  Copyright 2021 The Cerebro Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// fakeMarket is a Market whose only meaningful method is Candles, returning a fixed
// history for Warmup to seed from. The rest satisfy the interface as no-ops.
type fakeMarket struct{ candles indicator.Candles }

func (f fakeMarket) Stocks(context.Context) []*item.Item { return nil }
func (f fakeMarket) Candles(context.Context, string, market.CandleType) (indicator.Candles, error) {
	return f.candles, nil
}
func (f fakeMarket) Subscribe(context.Context, market.TickEventHandler) error { return nil }
func (f fakeMarket) Order(context.Context, order.Order) error                 { return nil }
func (f fakeMarket) AccountPositions(context.Context) []position.Position     { return nil }
func (f fakeMarket) AccountBalance(context.Context) decimal.Decimal           { return decimal.Zero }
func (f fakeMarket) Events(context.Context) <-chan any                        { return nil }
func (f fakeMarket) Commission() market.Rate                                  { return market.Fraction(decimal.Zero) }

// TestUniverseWarmup_SeedsThenFoldsLiveTicks verifies Warmup returns a stream warm
// from the adapter's history before any tick, then closes a live bar as ticks cross
// a bucket boundary, with History growing past the seed.
func TestUniverseWarmup_SeedsThenFoldsLiveTicks(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	seed := indicator.Candles{
		{Date: base, Code: "AAA", Close: decimal.NewFromInt(10)},
		{Date: base.Add(time.Minute), Code: "AAA", Close: decimal.NewFromInt(11)},
	}
	ticks := make(chan indicator.Tick, 8)
	u := &universe{
		items:  []*item.Item{{Code: "AAA"}},
		ticks:  ticks,
		market: fakeMarket{candles: seed},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs, err := u.Warmup(ctx, "AAA", market.Min)
	must.NoError(err)
	must.Len(cs.History(), 2) // seeded warm before any live tick

	// First tick opens the forming bar; the second, in the next minute bucket, closes it.
	ticks <- indicator.Tick{Date: base.Add(2 * time.Minute), Code: "AAA", Price: decimal.NewFromInt(12), Volume: 1}
	ticks <- indicator.Tick{Date: base.Add(3 * time.Minute), Code: "AAA", Price: decimal.NewFromInt(13), Volume: 1}

	select {
	case closed := <-cs.Closed():
		is.True(closed.Close.Equal(decimal.NewFromInt(12)), "closed bar holds the first live tick")
	case <-time.After(time.Second):
		t.Fatal("expected a closed bar on Closed()")
	}
	is.Eventually(func() bool { return len(cs.History()) == 3 }, time.Second, 10*time.Millisecond,
		"History should grow to seed(2) + 1 closed live bar")
}

// TestUniverseWarmup_MultiCodeCatchesUpBufferedTicks guards the fix where a second
// code's Warmup, registered after the dispatcher started, missed ticks that arrived
// in between. The first bar of the late-registered code must include the earlier tick.
func TestUniverseWarmup_MultiCodeCatchesUpBufferedTicks(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	ticks := make(chan indicator.Tick, 8)
	u := &universe{
		items:  []*item.Item{{Code: "AAA"}, {Code: "BBB"}},
		ticks:  ticks,
		market: fakeMarket{}, // no seed: exercise live folding + catch-up
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := u.Warmup(ctx, "AAA", market.Min) // starts the dispatcher
	must.NoError(err)

	// A BBB tick arrives before BBB's stream is registered: it is buffered (or still
	// in flight), and must not be lost.
	ticks <- indicator.Tick{Date: base, Code: "BBB", Price: decimal.NewFromInt(50), Volume: 1}

	csB, err := u.Warmup(ctx, "BBB", market.Min)
	must.NoError(err)

	// A tick in the next bucket closes BBB's first bar, which must reflect the earlier
	// price 50, not start fresh at 60.
	ticks <- indicator.Tick{Date: base.Add(time.Minute), Code: "BBB", Price: decimal.NewFromInt(60), Volume: 1}

	select {
	case closed := <-csB.Closed():
		is.True(closed.Close.Equal(decimal.NewFromInt(50)), "BBB's first bar must include the tick that predated its registration")
	case <-time.After(time.Second):
		t.Fatal("expected BBB's first bar to close")
	}
}

// TestUniverseWarmup_SameCodeSecondLevelCatchesUp guards the multi-timeframe case:
// warming the same code at a second level after the dispatcher started must still
// catch the second stream up from the rolling backlog, not miss the early ticks the
// first level already consumed.
func TestUniverseWarmup_SameCodeSecondLevelCatchesUp(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	ticks := make(chan indicator.Tick, 8)
	u := &universe{
		items:  []*item.Item{{Code: "AAA"}},
		ticks:  ticks,
		market: fakeMarket{}, // no seed: exercise live folding + catch-up
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := u.Warmup(ctx, "AAA", market.Min) // starts the dispatcher
	must.NoError(err)

	// A tick arrives for the Min stream and is recorded in the rolling backlog so a
	// later same-code stream can catch up (whether it was already buffered or is still
	// in flight when the second Warmup registers).
	ticks <- indicator.Tick{Date: base, Code: "AAA", Price: decimal.NewFromInt(50), Volume: 1}

	csMin5, err := u.Warmup(ctx, "AAA", market.Min5) // second timeframe, registered late
	must.NoError(err)

	// A tick five minutes later closes the Min5 bar [base, base+5m), which must include
	// the earlier price-50 tick caught up from the backlog, not start fresh at 60.
	ticks <- indicator.Tick{Date: base.Add(5 * time.Minute), Code: "AAA", Price: decimal.NewFromInt(60), Volume: 1}

	select {
	case closed := <-csMin5.Closed():
		is.True(closed.Close.Equal(decimal.NewFromInt(50)), "the Min5 bar must include the tick that predated its registration")
	case <-time.After(time.Second):
		t.Fatal("expected the Min5 bar to close")
	}
}

// TestUniverseWarmup_RejectsUnknownLevel verifies a level with no duration is
// rejected rather than producing a degenerate one-candle-per-tick resampler.
func TestUniverseWarmup_RejectsUnknownLevel(t *testing.T) {
	must := require.New(t)
	u := &universe{
		items:  []*item.Item{{Code: "AAA"}},
		ticks:  make(chan indicator.Tick),
		market: fakeMarket{},
	}
	_, err := u.Warmup(context.Background(), "AAA", market.CandleType(0))
	must.Error(err)
	must.ErrorContains(err, "unknown candle level")
}

// TestUniverseWarmup_RejectsDailyAndLonger verifies daily/weekly levels are rejected:
// fixed-width tick bucketing is epoch-aligned, so live bars would not continue the
// seeded calendar/exchange bars at the same boundaries.
func TestUniverseWarmup_RejectsDailyAndLonger(t *testing.T) {
	must := require.New(t)
	u := &universe{
		items:  []*item.Item{{Code: "AAA"}},
		ticks:  make(chan indicator.Tick),
		market: fakeMarket{},
	}
	for _, level := range []market.CandleType{market.Day, market.Week} {
		_, err := u.Warmup(context.Background(), "AAA", level)
		must.Error(err)
		must.ErrorContains(err, "unsupported")
	}
}
