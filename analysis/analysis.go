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

// Package analysis turns a broker's closed-trade log and equity curve into
// backtest/live performance metrics. Its functions are pure — they read the slices
// the broker accumulates and compute summary statistics — so they are simple to test
// and free of any trading state.
package analysis

import (
	"math"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/broker"
)

// Summary is the headline performance of a run: trade-level statistics from the
// closed-trade log and time-level statistics from the equity curve.
type Summary struct {
	// Trade-level (from the closed-trade log).
	Trades       int             // completed round-trips
	Wins         int             // trades with NetPnL > 0
	Losses       int             // trades with NetPnL < 0
	WinRate      float64         // Wins / Trades
	NetPnL       decimal.Decimal // sum of every trade's NetPnL (realized minus fees)
	GrossProfit  decimal.Decimal // sum of winning trades' NetPnL
	GrossLoss    decimal.Decimal // sum of losing trades' NetPnL as a positive magnitude
	ProfitFactor float64         // GrossProfit / GrossLoss (0 when there are no losses)
	Expectancy   decimal.Decimal // NetPnL / Trades — expected profit per trade
	AvgWin       decimal.Decimal // GrossProfit / Wins
	AvgLoss      decimal.Decimal // GrossLoss / Losses (positive magnitude)

	// Time-level (from the equity curve).
	StartEquity    decimal.Decimal
	EndEquity      decimal.Decimal
	TotalReturn    float64         // (EndEquity - StartEquity) / StartEquity
	MaxDrawdown    decimal.Decimal // largest peak-to-trough equity drop (absolute)
	MaxDrawdownPct float64         // that drop as a fraction of the peak it fell from
	Sharpe         float64         // annualized Sharpe of per-sample returns
}

// Summarize computes the full Summary from a closed-trade log and an equity curve.
// periodsPerYear annualizes the Sharpe ratio for the equity curve's sampling cadence
// (e.g. 252 for trading days, 365 for a 24/7 market sampled daily).
func Summarize(trades []broker.Trade, equity []broker.EquityPoint, periodsPerYear float64) Summary {
	s := Summary{}
	for i := range trades { // index to avoid copying the 160-byte Trade each iteration
		net := trades[i].NetPnL()
		s.NetPnL = s.NetPnL.Add(net)
		switch {
		case net.GreaterThan(decimal.Zero):
			s.Wins++
			s.GrossProfit = s.GrossProfit.Add(net)
		case net.LessThan(decimal.Zero):
			s.Losses++
			s.GrossLoss = s.GrossLoss.Sub(net) // net is negative; accumulate magnitude
		}
	}
	s.Trades = len(trades)
	if s.Trades > 0 {
		s.WinRate = float64(s.Wins) / float64(s.Trades)
		s.Expectancy = s.NetPnL.Div(decimal.NewFromInt(int64(s.Trades)))
	}
	if s.Wins > 0 {
		s.AvgWin = s.GrossProfit.Div(decimal.NewFromInt(int64(s.Wins)))
	}
	if s.Losses > 0 {
		s.AvgLoss = s.GrossLoss.Div(decimal.NewFromInt(int64(s.Losses)))
	}
	if s.GrossLoss.GreaterThan(decimal.Zero) {
		s.ProfitFactor = s.GrossProfit.Div(s.GrossLoss).InexactFloat64()
	}

	if len(equity) > 0 {
		s.StartEquity = equity[0].Equity
		s.EndEquity = equity[len(equity)-1].Equity
		s.TotalReturn = TotalReturn(equity)
		s.MaxDrawdown, s.MaxDrawdownPct = MaxDrawdown(equity)
		s.Sharpe = Sharpe(equity, periodsPerYear)
	}
	return s
}

// MaxDrawdown returns the largest peak-to-trough drop in the equity curve, both as an
// absolute amount and as a fraction of the peak it fell from. An empty or
// monotonically rising curve has zero drawdown.
func MaxDrawdown(equity []broker.EquityPoint) (maxDD decimal.Decimal, ddPct float64) {
	peak := decimal.Zero
	for i, p := range equity {
		if i == 0 || p.Equity.GreaterThan(peak) {
			peak = p.Equity
		}
		dd := peak.Sub(p.Equity)
		if dd.GreaterThan(maxDD) {
			maxDD = dd
			if peak.GreaterThan(decimal.Zero) {
				ddPct = dd.Div(peak).InexactFloat64()
			}
		}
	}
	return maxDD, ddPct
}

// TotalReturn is the simple return of the equity curve, (end - start) / start. It is
// zero when there are fewer than two points or the start equity is non-positive.
func TotalReturn(equity []broker.EquityPoint) float64 {
	if len(equity) < 2 {
		return 0
	}
	start := equity[0].Equity
	if start.LessThanOrEqual(decimal.Zero) {
		return 0
	}
	return equity[len(equity)-1].Equity.Sub(start).Div(start).InexactFloat64()
}

// Sharpe is the annualized Sharpe ratio of the equity curve's per-sample simple
// returns (excess over a zero risk-free rate), scaled by sqrt(periodsPerYear). It is
// zero when there are fewer than two returns or their standard deviation is zero.
func Sharpe(equity []broker.EquityPoint, periodsPerYear float64) float64 {
	if len(equity) < 3 { // need >= 2 returns for a sample standard deviation
		return 0
	}
	returns := make([]float64, 0, len(equity)-1)
	for i := 1; i < len(equity); i++ {
		prev := equity[i-1].Equity
		if prev.LessThanOrEqual(decimal.Zero) {
			continue
		}
		returns = append(returns, equity[i].Equity.Sub(prev).Div(prev).InexactFloat64())
	}
	if len(returns) < 2 {
		return 0
	}
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))
	var variance float64
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(returns) - 1) // sample variance
	sd := math.Sqrt(variance)
	if sd == 0 {
		return 0
	}
	return mean / sd * math.Sqrt(periodsPerYear)
}
