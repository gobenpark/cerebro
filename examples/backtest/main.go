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

// Command backtest is a runnable demo of cerebro driven by the replay market.
// It replays a short price series through a simple dip-buying strategy guarded by
// a risk gate, then prints the resulting balance and positions.
//
//	go run ./examples/backtest
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

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

// dipBuyer buys once the price dips a threshold below the first price it sees,
// then holds. Minimal, just enough to show the full tick -> order -> fill loop.
type dipBuyer struct {
	name   string
	first  decimal.Decimal
	bought bool
}

func (s *dipBuyer) Name() string { return s.name }

func (s *dipBuyer) Run(ctx context.Context, u strategy.Universe, b broker.Submitter) {
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
			if s.first.IsZero() {
				s.first = tk.Price
			}
			// Buy 10 units once the price is 2% under the opening price.
			if !s.bought && tk.Price.LessThan(s.first.Mul(decimal.NewFromFloat(0.98))) {
				o := order.NewOrder(it, order.Buy, order.Limit, decimal.NewFromInt(10), tk.Price)
				if err := b.Order(ctx, o, true); err == nil {
					s.bought = true
				}
			}
		}
	}
}

func (s *dipBuyer) NotifyOrder(o order.Order) {
	fmt.Printf("  notify [%s] %s status=%v\n", s.name, o.Item().Code, o.Status())
}

func (s *dipBuyer) NotifyTrade() {}
func (s *dipBuyer) NotifyFund()  {}

// series builds candles whose OHLC all equal the given closing prices.
func series(code string, prices ...int64) indicator.Candles {
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	cds := make(indicator.Candles, len(prices))
	for i, p := range prices {
		d := decimal.NewFromInt(p)
		cds[i] = &indicator.Candle{
			Code:   code,
			Date:   base.Add(time.Duration(i) * time.Minute),
			Open:   d,
			High:   d,
			Low:    d,
			Close:  d,
			Volume: 1,
		}
	}
	return cds
}

func main() {
	// A series that opens at 100, dips to 97, then recovers to 103.
	prices := []int64{100, 100, 99, 98, 97, 97, 97, 98, 99, 100, 101, 102, 103}

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(1_000_000)),
		replay.WithCommission(market.Percent(decimal.NewFromFloat(0.015))), // 0.015%
		replay.WithInterval(10*time.Millisecond),
		replay.WithCandles("AAA", series("AAA", prices...)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&dipBuyer{name: "dip"}),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}),
		// Risk gate is active (here the order is well within limits, so it passes).
		cerebro.WithRisk(
			risk.MaxPositionPct(0.5),
			risk.MaxOrderValue(decimal.NewFromInt(500_000)),
		),
		cerebro.WithLogLevel(slog.LevelError),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("running backtest over", len(prices), "bars ...")
	if err := cb.Start(ctx); err != nil {
		panic(err)
	}

	<-mkt.Done() // wait for the replay to finish
	cancel()
	cb.Shutdown()

	fmt.Printf("\nfinal balance: %s\n", mkt.AccountBalance(context.Background()).StringFixed(2))
	positions := mkt.AccountPositions(context.Background())
	if len(positions) == 0 {
		fmt.Println("no open positions")
	}
	for _, p := range positions {
		fmt.Printf("position %s size=%s avg=%s\n", p.Item.Code, p.Size, p.Price.StringFixed(2))
	}
}
