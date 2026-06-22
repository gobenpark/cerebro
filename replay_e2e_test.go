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
)

// buyOnceStrategy places a single limit buy on the first tick it receives, then
// holds. It is the minimal strategy needed to exercise the full pipeline.
type buyOnceStrategy struct{ once sync.Once }

func (s *buyOnceStrategy) Name() string { return "buy-once" }

func (s *buyOnceStrategy) Next(ctx context.Context, it *item.Item, tick <-chan indicator.Tick, b *broker.Broker) {
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
