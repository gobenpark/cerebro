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

	"github.com/gobenpark/cerebro"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market/replay"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/risk"
)

// buyOnceStrategy places a single limit buy on the first tick it receives, then
// holds. It is the minimal strategy needed to exercise the full pipeline.
type buyOnceStrategy struct{ once sync.Once }

func (s *buyOnceStrategy) Name() string { return "buy-once" }

func (s *buyOnceStrategy) Next(ctx context.Context, it *item.Item, tick <-chan indicator.Tick, b broker.Submitter) {
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
		replay.WithCommission(decimal.Zero),
		replay.WithInterval(20*time.Millisecond),
		replay.WithCandles("AAA", flatCandles("AAA", 40, 100)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&buyOnceStrategy{}),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		cerebro.WithLogLevel(log.FatalLevel),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	// The limit buy (10 @ 100) fills against the flat 100 price, debiting 1000.
	is.Eventually(func() bool {
		return mkt.AccountBalance().Equal(decimal.NewFromInt(99_000))
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

	// Hold at 100 long enough for the buy to fill, then drop to 94 (below the 5%
	// stop at 95) and stay there so the market exit fills.
	prices := append(repeat(100, 5), repeat(94, 30)...)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(100_000)),
		replay.WithCommission(decimal.Zero),
		replay.WithInterval(15*time.Millisecond),
		replay.WithCandles("AAA", seriesCandles("AAA", prices...)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&buyOnceStrategy{}),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		cerebro.WithRiskPolicy("buy-once", risk.Policy{StopLoss: 0.05}),
		cerebro.WithLogLevel(log.FatalLevel),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	// Buy 10 @ 100 (-1000); the stop fires at 94, exiting 10 @ 94 (+940). Net cash
	// is 99_940 and no position remains.
	is.Eventually(func() bool {
		return mkt.AccountBalance().Equal(decimal.NewFromInt(99_940)) &&
			len(mkt.AccountPositions()) == 0
	}, 5*time.Second, 15*time.Millisecond, "stop-loss should exit the position via the replay market")

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
		cerebro.WithStrategy(&buyOnceStrategy{}), // Name() == "buy-once"
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		cerebro.WithRiskPolicy("typo", risk.Policy{StopLoss: 0.05}),
		cerebro.WithLogLevel(log.FatalLevel),
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
		cerebro.WithStrategy(&buyOnceStrategy{}),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		cerebro.WithRiskPolicy("nobody", risk.Policy{}), // empty == disabled
		cerebro.WithLogLevel(log.FatalLevel),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx), "a disabled policy for an unknown strategy must be inert")

	cancel()
	cb.Shutdown()
}
