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
package cerebro_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"log/slog"

	"github.com/gobenpark/cerebro"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/market/replay"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

// buyOnceStrategy places a single limit buy on the first tick it receives, then
// holds. It is the minimal strategy needed to exercise the full pipeline.
type buyOnceStrategy struct{ once sync.Once }

func (s *buyOnceStrategy) Name() string { return "buy-once" }

func (s *buyOnceStrategy) Run(ctx context.Context, u strategy.Universe, b broker.Submitter) {
	it := u.Items()[0]
	tick := u.Ticks()
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-tick:
			if !ok {
				return
			}
			s.once.Do(func() {
				o := order.NewOrder(it, order.Buy, order.Limit, decimal.NewFromInt(10), tk.Price)
				_ = b.Order(ctx, o, false)
			})
		}
	}
}

func (s *buyOnceStrategy) NotifyOrder(order.Order) {}
func (s *buyOnceStrategy) NotifyTrade()            {}
func (s *buyOnceStrategy) NotifyFund()             {}

func flatCandles(code string, n int, price int64) indicator.Candles {
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	cds := make(indicator.Candles, n)
	for i := range n {
		cds[i] = &indicator.Candle{
			Code:   code,
			Date:   base.Add(time.Duration(i) * time.Minute),
			Open:   decimal.NewFromInt(price),
			High:   decimal.NewFromInt(price),
			Low:    decimal.NewFromInt(price),
			Close:  decimal.NewFromInt(price),
			Volume: 1,
		}
	}
	return cds
}

// TestCerebro_ReplayEndToEnd runs the whole stack against the replay market: the
// strategy receives ticks, places a limit buy, the broker submits it, and the
// replay market fills it — observable as a drop in the simulated balance.
func TestCerebro_ReplayEndToEnd(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(20*time.Millisecond),
		replay.WithCandles("AAA", flatCandles("AAA", 40, 100)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&buyOnceStrategy{}, "AAA"),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	// The limit buy (10 @ 100) fills against the flat 100 price, debiting 1000.
	is.Eventually(func() bool {
		return mkt.AccountBalance(context.Background()).Equal(decimal.NewFromInt(99_000))
	}, 5*time.Second, 20*time.Millisecond, "strategy order should fill through the replay market")

	cancel()
	cb.Shutdown()
}

func repeat(price int64, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = price
	}
	return out
}

func seriesCandles(code string, prices ...int64) indicator.Candles {
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	cds := make(indicator.Candles, len(prices))
	for i, p := range prices {
		d := decimal.NewFromInt(p)
		cds[i] = &indicator.Candle{
			Code: code, Date: base.Add(time.Duration(i) * time.Minute),
			Open: d, High: d, Low: d, Close: d, Volume: 1,
		}
	}
	return cds
}

// TestCerebro_RiskPolicyExits drives the whole stack with a reactive stop-loss
// policy: the strategy buys and holds, the price then falls through the stop, and
// the monitor — registered by Cerebro and using the strategy's attribution —
// submits a market exit that the replay market fills. The position returns to flat
// and the cash from the exit is credited back.
func TestCerebro_RiskPolicyExits(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	// Hold at 100 long enough for the buy to fill, then drop to 94 (below the 5% stop
	// at 95) and stay there for a long tail so the reactive exit — which is submitted
	// only after the fill propagates to the broker and back to the monitor — still has
	// candles left to fill against before the replay finishes.
	prices := append(repeat(100, 5), repeat(94, 80)...)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(15*time.Millisecond),
		replay.WithCandles("AAA", seriesCandles("AAA", prices...)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&buyOnceStrategy{}, "AAA"),
		cerebro.WithRiskPolicy("buy-once", risk.Policy{StopLoss: 0.05}),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	// Buy 10 @ 100 (-1000); the stop fires at 94, exiting 10 @ 94 (+940), booked as
	// (94-100)*10 = -60 realized PnL with a flat position. Poll the broker ledger
	// (the true downstream of the round trip) rather than the replay's own internal
	// balance, which is updated synchronously in matchAndFill and so leads the async
	// event that reaches the broker.
	is.Eventually(func() bool {
		rep := cb.Report()
		if len(rep) != 1 || rep[0].Strategy != "buy-once" {
			return false
		}
		return decimal.NewFromInt(-60).Equal(rep[0].Realized) && len(rep[0].Positions) == 0
	}, 5*time.Second, 15*time.Millisecond, "stop-loss should exit the position and book -60 realized PnL")

	cancel()
	cb.Shutdown()
}

// TestStart_RejectsPolicyForUnknownStrategy guards the wiring check: a policy that
// names a strategy that was never registered is a configuration error.
func TestStart_RejectsPolicyForUnknownStrategy(t *testing.T) {
	must := require.New(t)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCandles("AAA", seriesCandles("AAA", 100, 100)),
	)
	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&buyOnceStrategy{}, "AAA"), // Name() == "buy-once"
		cerebro.WithRiskPolicy("typo", risk.Policy{StopLoss: 0.05}),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	err := cb.Start(context.Background())
	must.Error(err)
	must.ErrorContains(err, "typo")
}

// TestStart_IgnoresDisabledPolicyForUnknownStrategy guards that a zero-valued
// (disabled) policy is inert: it is dropped rather than validated, so an unknown
// name on a disabled policy is not a startup error.
func TestStart_IgnoresDisabledPolicyForUnknownStrategy(t *testing.T) {
	must := require.New(t)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCandles("AAA", seriesCandles("AAA", 100, 100)),
	)
	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&buyOnceStrategy{}, "AAA"),
		cerebro.WithRiskPolicy("nobody", risk.Policy{}), // empty == disabled
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx), "a disabled policy for an unknown strategy must be inert")

	cancel()
	cb.Shutdown()
}

// TestCerebro_DisabledOverrideClearsRiskAware verifies that an explicit disabled
// (empty) WithRiskPolicy turns OFF a strategy-declared ExitPolicy: the price clears
// the declared +5% take-profit, but with the override the monitor must NOT exit, so
// the position is held and no PnL is realized.
func TestCerebro_DisabledOverrideClearsRiskAware(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	prices := append(repeat(100, 5), repeat(110, 60)...) // long tail past the take-profit

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(10*time.Millisecond),
		replay.WithCandles("AAA", seriesCandles("AAA", prices...)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithScreener(listScreener{&item.Item{Code: "AAA"}}, func(it *item.Item) strategy.Strategy {
			return &riskAwareBuyOnce{name: "ra:" + it.Code, policy: risk.Policy{TakeProfit: 0.05}}
		}),
		cerebro.WithRiskPolicy("ra:AAA", risk.Policy{}), // explicit disable clears the declared take-profit
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	// The buy (10 @ 100) fills and is held.
	is.Eventually(func() bool {
		rep := cb.Report()
		return len(rep) == 1 && rep[0].Strategy == "ra:AAA" &&
			len(rep[0].Positions) == 1 && rep[0].Positions[0].Size.Equal(decimal.NewFromInt(10))
	}, 5*time.Second, 10*time.Millisecond, "the buy should fill and be held")

	// With the declared take-profit cleared by the disabled override, the rise to 110
	// must NOT trigger an exit — the position stays held and realized stays zero through
	// the whole tail (a firing take-profit would flatten it and book PnL).
	is.Never(func() bool {
		rep := cb.Report()
		return len(rep) == 1 && (len(rep[0].Positions) == 0 || !rep[0].Realized.IsZero())
	}, time.Second, 20*time.Millisecond, "disabled override must not let the take-profit fire")

	cancel()
	cb.Shutdown()
}

// riskAwareBuyOnce buys once and declares its own exit policy via strategy.RiskAware,
// so Cerebro registers it with the monitor WITHOUT a name-based WithRiskPolicy —
// the path that reaches WithStrategyForEach, whose instance names aren't known until
// spawn.
type riskAwareBuyOnce struct {
	buyOnceStrategy
	name   string
	policy risk.Policy
}

func (s *riskAwareBuyOnce) Name() string            { return s.name }
func (s *riskAwareBuyOnce) ExitPolicy() risk.Policy { return s.policy }

var _ strategy.RiskAware = (*riskAwareBuyOnce)(nil)

// TestCerebro_RiskAwareExits_ForEach verifies the RiskAware path end-to-end: a
// per-item strategy spawned by WithStrategyForEach declares a take-profit through
// ExitPolicy (no WithRiskPolicy, since the name "ra:AAA" isn't known until spawn).
// Cerebro's buildMonitor must pick it up so the monitor exits the position when the
// price clears the take-profit, booking the gain as realized PnL.
func TestCerebro_RiskAwareExits_ForEach(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	// Fill at 100, then rise to 110 — past the +5% take-profit at 105 — and hold so
	// the reactive exit (submitted only after the fill round-trips to the monitor)
	// still has candles to fill against.
	prices := append(repeat(100, 5), repeat(110, 80)...)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(15*time.Millisecond),
		replay.WithCandles("AAA", seriesCandles("AAA", prices...)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithScreener(listScreener{&item.Item{Code: "AAA"}}, func(it *item.Item) strategy.Strategy {
			return &riskAwareBuyOnce{name: "ra:" + it.Code, policy: risk.Policy{TakeProfit: 0.05}}
		}),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	// Buy 10 @ 100 (-1000); take-profit fires at 110, exiting 10 @ 110 (+1100), booked
	// as (110-100)*10 = +100 realized PnL with a flat position.
	is.Eventually(func() bool {
		rep := cb.Report()
		if len(rep) != 1 || rep[0].Strategy != "ra:AAA" {
			return false
		}
		return decimal.NewFromInt(100).Equal(rep[0].Realized) && len(rep[0].Positions) == 0
	}, 5*time.Second, 15*time.Millisecond, "RiskAware take-profit should exit and book +100 realized PnL")

	cancel()
	cb.Shutdown()
}
