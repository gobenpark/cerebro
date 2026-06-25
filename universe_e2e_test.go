/*
 *  Copyright 2023 The Cerebro Authors
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
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/market/replay"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/strategy"
)

// pairBuyer is a minimal multi-asset strategy: it buys each code in its universe
// once, on the first tick it sees for that code. It proves a single Run observes
// every leg of its universe and can trade them under one attribution name. Run is
// driven from a single goroutine, so its map needs no locking.
type pairBuyer struct {
	bought map[string]bool
}

func (s *pairBuyer) Name() string { return "pairs" }

func (s *pairBuyer) Run(ctx context.Context, u strategy.Universe, b broker.Submitter) {
	s.bought = map[string]bool{}
	byCode := map[string]*item.Item{}
	for _, it := range u.Items() {
		byCode[it.Code] = it
	}
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-u.Ticks():
			if !ok {
				return
			}
			if s.bought[tk.Code] {
				continue
			}
			o := order.NewOrder(byCode[tk.Code], order.Buy, order.Limit, decimal.NewFromInt(5), tk.Price)
			if err := b.Order(ctx, o, false); err == nil {
				s.bought[tk.Code] = true
			}
		}
	}
}

func (s *pairBuyer) NotifyOrder(order.Order) {}
func (s *pairBuyer) NotifyTrade()            {}
func (s *pairBuyer) NotifyFund()             {}

// TestCerebro_PortfolioStrategyTradesBothLegs registers one strategy over a
// two-code universe and verifies it sees both legs and books a position in each,
// all attributed to its single name.
func TestCerebro_PortfolioStrategyTradesBothLegs(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(1_000_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(15*time.Millisecond),
		replay.WithCandles("AAA", flatCandles("AAA", 40, 100)),
		replay.WithCandles("BBB", flatCandles("BBB", 40, 200)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&pairBuyer{}, "AAA", "BBB"),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	is.Eventually(func() bool {
		rep := cb.Report()
		return len(rep) == 1 && rep[0].Strategy == "pairs" && len(rep[0].Positions) == 2
	}, 5*time.Second, 15*time.Millisecond, "one strategy should hold a position in each leg of its universe")

	rep := cb.Report()
	must.Len(rep[0].Positions, 2)
	// Positions are sorted by code: AAA then BBB.
	is.Equal("AAA", rep[0].Positions[0].Item.Code)
	is.True(decimal.NewFromInt(5).Equal(rep[0].Positions[0].Size))
	is.Equal("BBB", rep[0].Positions[1].Item.Code)
	is.True(decimal.NewFromInt(5).Equal(rep[0].Positions[1].Size))

	cancel()
	cb.Shutdown()
}

// codeBuyer buys its single universe item once, then holds.
type codeBuyer struct {
	name string
	once sync.Once
}

func (s *codeBuyer) Name() string { return s.name }

func (s *codeBuyer) Run(ctx context.Context, u strategy.Universe, b broker.Submitter) {
	it := u.Items()[0]
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-u.Ticks():
			if !ok {
				return
			}
			s.once.Do(func() {
				o := order.NewOrder(it, order.Buy, order.Limit, decimal.NewFromInt(3), tk.Price)
				_ = b.Order(ctx, o, false)
			})
		}
	}
}

func (s *codeBuyer) NotifyOrder(order.Order) {}
func (s *codeBuyer) NotifyTrade()            {}
func (s *codeBuyer) NotifyFund()             {}

// TestCerebro_ForEachReplicatesPerItem verifies WithStrategyForEach instantiates
// one strategy per watchlist with isolated state and attribution: each instance
// trades only its own code and reports under its own name.
func TestCerebro_ForEachReplicatesPerItem(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(1_000_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(15*time.Millisecond),
		replay.WithCandles("AAA", flatCandles("AAA", 40, 100)),
		replay.WithCandles("BBB", flatCandles("BBB", 40, 200)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithScreener(cerebro.StaticScreener(&item.Item{Code: "AAA"}, &item.Item{Code: "BBB"}), func(it *item.Item) strategy.Strategy {
			return &codeBuyer{name: "buy:" + it.Code}
		}),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	is.Eventually(func() bool {
		rep := cb.Report()
		return len(rep) == 2 &&
			len(rep[0].Positions) == 1 && len(rep[1].Positions) == 1
	}, 5*time.Second, 15*time.Millisecond, "each watchlist should get its own attributed instance")

	rep := cb.Report()
	// Report is sorted by strategy name: buy:AAA then buy:BBB.
	is.Equal("buy:AAA", rep[0].Strategy)
	is.Equal("AAA", rep[0].Positions[0].Item.Code)
	is.Equal("buy:BBB", rep[1].Strategy)
	is.Equal("BBB", rep[1].Positions[0].Item.Code)

	cancel()
	cb.Shutdown()
}

// TestCerebro_MultipleStrategiesShareSymbol guards the regression where two
// runners over the same code missed early ticks: with per-runner subscribe, the
// feed for the shared code started when the first runner subscribed, so the second
// runner (registered/started later) never saw the early ticks and could fail to
// trade in a short backtest. Both strategies must end up holding a position.
func TestCerebro_MultipleStrategiesShareSymbol(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(1_000_000)),
		replay.WithCommission(market.Fraction(decimal.Zero)),
		replay.WithInterval(15*time.Millisecond),
		replay.WithCandles("AAA", flatCandles("AAA", 40, 100)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		// Two strategies over the same single watchlist — they share code AAA.
		cerebro.WithStrategy(&codeBuyer{name: "share-a"}, "AAA"),
		cerebro.WithStrategy(&codeBuyer{name: "share-b"}, "AAA"),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(cb.Start(ctx))

	is.Eventually(func() bool {
		rep := cb.Report()
		return len(rep) == 2 && len(rep[0].Positions) == 1 && len(rep[1].Positions) == 1
	}, 5*time.Second, 15*time.Millisecond, "both strategies sharing a symbol must receive ticks and trade")

	cancel()
	cb.Shutdown()
}

// TestStart_RejectsDuplicateUniverseCode guards that a universe listing the same
// code twice is rejected, so a runner's channel is never registered twice under a
// code (which would deliver every tick to the strategy twice).
func TestStart_RejectsDuplicateUniverseCode(t *testing.T) {
	must := require.New(t)

	mkt := replay.New(replay.WithBalance(decimal.NewFromInt(100_000)))
	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(dupStub{name: "s"}, "AAA", "AAA"),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	err := cb.Start(context.Background())
	must.Error(err)
	must.ErrorContains(err, "duplicate code")
}

// dupStub is a stub strategy with a caller-chosen name, used to force a name clash.
// id only distinguishes instances; Name() (not id) is what must collide.
type dupStub struct{ id, name string }

func (s dupStub) Name() string                                                     { return s.name }
func (s dupStub) Run(ctx context.Context, _ strategy.Universe, _ broker.Submitter) { <-ctx.Done() }
func (s dupStub) NotifyOrder(order.Order)                                          {}
func (s dupStub) NotifyTrade()                                                     {}
func (s dupStub) NotifyFund()                                                      {}

// TestStart_RejectsDuplicateStrategyName guards the uniqueness check: two
// strategies sharing a Name() would mis-route order notifications, so Start fails.
func TestStart_RejectsDuplicateStrategyName(t *testing.T) {
	must := require.New(t)

	mkt := replay.New(replay.WithBalance(decimal.NewFromInt(100_000)))
	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(dupStub{id: "a", name: "same"}, "AAA"),
		cerebro.WithStrategy(dupStub{id: "b", name: "same"}, "AAA"),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	err := cb.Start(context.Background())
	must.Error(err)
	must.ErrorContains(err, "duplicate")
}
