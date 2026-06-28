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
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/market/replay"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

// dipBuyer buys once the price dips a threshold below the first price it sees, then
// sells the position back once the price recovers to the opening level — a complete
// round-trip, so the run produces a closed trade for the performance report.
type dipBuyer struct {
	name   string
	first  decimal.Decimal
	bought bool
	sold   bool
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
			switch {
			// Buy 10 units once the price is 2% under the opening price.
			case !s.bought && tk.Price.LessThan(s.first.Mul(decimal.NewFromFloat(0.98))):
				o := order.NewOrder(it, order.Buy, order.Limit, decimal.NewFromInt(40), tk.Price)
				if err := b.Order(ctx, o, true); err == nil {
					s.bought = true
				}
			// Sell back once the price recovers to the opening level — closes the trade.
			case s.bought && !s.sold && tk.Price.GreaterThanOrEqual(s.first):
				o := order.NewOrder(it, order.Sell, order.Limit, decimal.NewFromInt(40), tk.Price)
				if err := b.Order(ctx, o, true); err == nil {
					s.sold = true
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

// series builds candles whose OHLC all equal the given closing prices, one per day
// so the run spans several calendar days and the equity curve is sampled daily —
// enough points for the drawdown and Sharpe figures in the performance report.
func series(code string, prices ...int64) indicator.Candles {
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	cds := make(indicator.Candles, len(prices))
	for i, p := range prices {
		d := decimal.NewFromInt(p)
		cds[i] = &indicator.Candle{
			Code:   code,
			Date:   base.AddDate(0, 0, i),
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
		replay.WithBalance(decimal.NewFromInt(10_000)),
		replay.WithCommission(market.Percent(decimal.NewFromFloat(0.015))), // 0.015%
		replay.WithInterval(10*time.Millisecond),
		replay.WithCandles("AAA", series("AAA", prices...)),
	)

	cb := cerebro.NewCerebro(
		cerebro.WithMarket(mkt),
		cerebro.WithStrategy(&dipBuyer{name: "dip"}, "AAA"),
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

	// Performance summary: trade-level stats from the closed-trade log and time-level
	// stats from the daily equity curve.
	p := cb.Performance()
	fmt.Printf("\nperformance\n")
	fmt.Printf("  trades=%d  winRate=%.0f%%  profitFactor=%.2f  netPnL=%s\n",
		p.Trades, p.WinRate*100, p.ProfitFactor, p.NetPnL.StringFixed(2))
	fmt.Printf("  maxDrawdown=%s (%.2f%%)  sharpe=%.2f  totalReturn=%.2f%%\n",
		p.MaxDrawdown.StringFixed(2), p.MaxDrawdownPct*100, p.Sharpe, p.TotalReturn*100)
}
