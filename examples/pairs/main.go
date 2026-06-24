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

// Command pairs is a runnable demo of a multi-asset (pairs) strategy. One strategy
// watches TWO instruments at once through its Universe, computes the A/B price
// ratio, and trades the A leg when the ratio diverges and reverts — something the
// single-instrument strategy model could not express.
//
//	go run ./examples/pairs
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
	"github.com/gobenpark/cerebro/strategy"
)

// spread is a long-only pairs strategy on the A/B ratio. It enters a long in A
// when A is cheap relative to B (ratio below the entry band) and exits when the
// ratio reverts. It is long-only because the replay market does not support
// shorting; a real pairs trade would also short the rich leg.
type spread struct {
	codeA, codeB string
	itemA        *item.Item
	lastA, lastB decimal.Decimal
	holding      bool
}

func (s *spread) Name() string { return "ab-spread" }

func (s *spread) Run(ctx context.Context, u strategy.Universe, b broker.Submitter) {
	// Resolve the A item once so exits/entries reference the right instrument.
	for _, it := range u.Items() {
		if it.Code == s.codeA {
			s.itemA = it
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-u.Ticks():
			if !ok {
				return
			}
			// One channel carries both legs; demultiplex by Code.
			switch tk.Code {
			case s.codeA:
				s.lastA = tk.Price
			case s.codeB:
				s.lastB = tk.Price
			}
			if s.lastA.IsZero() || s.lastB.IsZero() {
				continue // need both legs before deciding
			}

			ratio := s.lastA.Div(s.lastB)
			switch {
			case !s.holding && ratio.LessThan(decimal.NewFromFloat(0.95)):
				o := order.NewOrder(s.itemA, order.Buy, order.Limit, decimal.NewFromInt(10), s.lastA)
				if err := b.Order(ctx, o, true); err == nil {
					s.holding = true
					fmt.Printf("  enter long %s @ %s (ratio %s)\n", s.codeA, s.lastA, ratio.StringFixed(3))
				}
			case s.holding && ratio.GreaterThan(decimal.NewFromFloat(1.0)):
				o := order.NewOrder(s.itemA, order.Sell, order.Limit, decimal.NewFromInt(10), s.lastA)
				if err := b.Order(ctx, o, true); err == nil {
					s.holding = false
					fmt.Printf("  exit  long %s @ %s (ratio %s)\n", s.codeA, s.lastA, ratio.StringFixed(3))
				}
			}
		}
	}
}

func (s *spread) NotifyOrder(order.Order) {}
func (s *spread) NotifyTrade()            {}
func (s *spread) NotifyFund()             {}

// series builds candles whose OHLC all equal the given closing prices.
func series(code string, prices ...int64) indicator.Candles {
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

func flat(code string, n int, price int64) indicator.Candles {
	prices := make([]int64, n)
	for i := range prices {
		prices[i] = price
	}
	return series(code, prices...)
}

func main() {
	// BBB holds at 100; AAA dips (ratio falls below 0.95 -> enter) then recovers
	// above parity (ratio rises above 1.0 -> exit).
	aaa := series("AAA", 100, 100, 95, 90, 90, 95, 100, 103, 105, 105, 105, 105, 105)
	bbb := flat("BBB", 13, 100)

	mkt := replay.New(
		replay.WithBalance(decimal.NewFromInt(1_000_000)),
		replay.WithCommission(market.Percent(decimal.NewFromFloat(0.015))),
		replay.WithInterval(10*time.Millisecond),
		replay.WithCandles("AAA", aaa),
		replay.WithCandles("BBB", bbb),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		// One strategy, two-instrument universe — this is the pairs entry point.
		cerebro.WithStrategy(&spread{codeA: "AAA", codeB: "BBB"}, "AAA", "BBB"),
		cerebro.WithTargetItem(&item.Item{Code: "AAA"}, &item.Item{Code: "BBB"}),
		cerebro.WithLogLevel(slog.LevelError),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("running pairs backtest (AAA/BBB ratio) ...")
	if err := cb.Start(ctx); err != nil {
		panic(err)
	}

	<-mkt.Done()
	cancel()
	cb.Shutdown()

	fmt.Printf("\nfinal balance: %s\n", mkt.AccountBalance(context.Background()).StringFixed(2))
	for _, r := range cb.Report() {
		fmt.Printf("strategy %s realized=%s fees=%s\n", r.Strategy, r.Realized.StringFixed(2), r.Fees.StringFixed(2))
	}
}
