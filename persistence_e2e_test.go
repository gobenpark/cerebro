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
	"github.com/gobenpark/cerebro/store"
	"github.com/gobenpark/cerebro/strategy"
)

// holdStrategy only drains ticks; it never trades. The second leg of a restart
// test uses it so the restored ledger is observed, not perturbed by new orders.
type holdStrategy struct{}

func (holdStrategy) Name() string { return "observer" }

func (holdStrategy) Run(ctx context.Context, u strategy.Universe, _ broker.Submitter) {
	tick := u.Ticks()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-tick:
			if !ok {
				return
			}
		}
	}
}

func (holdStrategy) NotifyOrder(order.Order) {}
func (holdStrategy) NotifyTrade()            {}
func (holdStrategy) NotifyFund()             {}

// TestCerebro_PersistsAndRestoresLedgerAcrossRestart runs the whole stack twice
// against a shared store. The first run buys and holds, persisting an open lot;
// the second run — a fresh Cerebro that only observes — restores that lot on
// Start, before any new event flows, proving per-strategy state survives a
// restart even though the (replay) exchange reports no position of its own.
func TestCerebro_PersistsAndRestoresLedgerAcrossRestart(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	st := store.NewMemoryStorage()

	candles := func() indicator.Candles { return flatCandles("AAA", 40, 100) }

	// Run 1: buy 10 @ 100 and hold; the fill persists an open lot to the store.
	cb1 := cerebro.NewCerebro(
		cerebro.WithMarket(replay.New(
			replay.WithBalance(decimal.NewFromInt(100_000)),
			replay.WithCommission(market.Fraction(decimal.Zero)),
			replay.WithInterval(20*time.Millisecond),
			replay.WithCandles("AAA", candles()),
		)),
		cerebro.WithStrategy(&buyOnceStrategy{}),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		cerebro.WithStorage(st),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx1, cancel1 := context.WithCancel(context.Background())
	must.NoError(cb1.Start(ctx1))
	is.Eventually(func() bool {
		rep := cb1.Report()
		return len(rep) == 1 && len(rep[0].Positions) == 1 &&
			decimal.NewFromInt(10).Equal(rep[0].Positions[0].Size)
	}, 5*time.Second, 20*time.Millisecond, "the buy should fill and persist an open lot")
	cancel1()
	cb1.Shutdown()

	// Run 2: a fresh stack that only observes. Restore runs inside Start, so the
	// prior open lot is present the moment Start returns.
	cb2 := cerebro.NewCerebro(
		cerebro.WithMarket(replay.New(
			replay.WithBalance(decimal.NewFromInt(100_000)),
			replay.WithCommission(market.Fraction(decimal.Zero)),
			replay.WithInterval(20*time.Millisecond),
			replay.WithCandles("AAA", candles()),
		)),
		cerebro.WithStrategy(holdStrategy{}),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		cerebro.WithStorage(st),
		cerebro.WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	must.NoError(cb2.Start(ctx2))

	rep := cb2.Report()
	must.Len(rep, 1, "the persisted ledger should be restored on start")
	is.Equal("buy-once", rep[0].Strategy, "attribution survives the restart")
	must.Len(rep[0].Positions, 1)
	is.True(decimal.NewFromInt(10).Equal(rep[0].Positions[0].Size), "held size restored")
	is.True(decimal.NewFromInt(100).Equal(rep[0].Positions[0].Price), "average entry restored")

	cancel2()
	cb2.Shutdown()
}
