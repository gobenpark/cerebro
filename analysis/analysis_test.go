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

package analysis_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/broker"
)

func dec(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

func trade(realized, fees int64) broker.Trade {
	return broker.Trade{Realized: dec(realized), Fees: dec(fees)}
}

func equity(vals ...int64) []broker.EquityPoint {
	pts := make([]broker.EquityPoint, len(vals))
	for i, v := range vals {
		pts[i] = broker.EquityPoint{Equity: dec(v)}
	}
	return pts
}

// TestSummarize_TradeStats checks win rate, profit factor, expectancy, and average
// win/loss from a closed-trade log: two wins (net 90, 40) and one loss (net -40).
func TestSummarize_TradeStats(t *testing.T) {
	is := assert.New(t)

	s := analysis.Summarize([]broker.Trade{
		trade(100, 10), // net +90 win
		trade(50, 10),  // net +40 win
		trade(-30, 10), // net -40 loss
	}, nil, 365)

	is.Equal(3, s.Trades)
	is.Equal(2, s.Wins)
	is.Equal(1, s.Losses)
	is.InDelta(2.0/3.0, s.WinRate, 1e-9)
	is.True(dec(90).Equal(s.NetPnL), "90 + 40 - 40")
	is.True(dec(130).Equal(s.GrossProfit))
	is.True(dec(40).Equal(s.GrossLoss), "loss magnitude is positive")
	is.InDelta(3.25, s.ProfitFactor, 1e-9) // 130 / 40
	is.True(dec(30).Equal(s.Expectancy))   // 90 / 3
	is.True(dec(65).Equal(s.AvgWin))       // 130 / 2
	is.True(dec(40).Equal(s.AvgLoss))
}

func TestSummarize_EmptyIsZero(t *testing.T) {
	is := assert.New(t)
	s := analysis.Summarize(nil, nil, 365)
	is.Equal(0, s.Trades)
	is.Zero(s.WinRate)
	is.Zero(s.ProfitFactor)
	is.True(s.NetPnL.IsZero())
}

func TestSummarize_NoLossesLeavesProfitFactorZero(t *testing.T) {
	is := assert.New(t)
	s := analysis.Summarize([]broker.Trade{trade(50, 0), trade(20, 0)}, nil, 365)
	is.Equal(2, s.Wins)
	is.Equal(0, s.Losses)
	is.Zero(s.ProfitFactor, "undefined with no losses -> 0, caller sees Losses == 0")
}

func TestMaxDrawdown(t *testing.T) {
	is := assert.New(t)

	dd, pct := analysis.MaxDrawdown(equity(100, 120, 90, 110))
	is.True(dec(30).Equal(dd), "peak 120 -> trough 90")
	is.InDelta(0.25, pct, 1e-9) // 30 / 120

	zero, zpct := analysis.MaxDrawdown(equity(100, 110, 120)) // monotonic rise
	is.True(zero.IsZero())
	is.Zero(zpct)
}

func TestTotalReturn(t *testing.T) {
	is := assert.New(t)
	is.InDelta(0.1, analysis.TotalReturn(equity(100, 90, 110)), 1e-9) // (110-100)/100
	is.Zero(analysis.TotalReturn(equity(100)), "fewer than two points -> 0")
}

func TestSharpe(t *testing.T) {
	is := assert.New(t)

	// Returns of [100,120,90,110]: 0.2, -0.25, 0.22222; mean 0.0574074,
	// sample sd 0.2664544; Sharpe (periodsPerYear=1) = 0.215452.
	s := analysis.Sharpe(equity(100, 120, 90, 110), 1)
	is.InDelta(0.215452, s, 1e-5)

	is.Zero(analysis.Sharpe(equity(100, 110), 1), "fewer than two returns -> 0")
	is.Zero(analysis.Sharpe(equity(100, 100, 100), 1), "zero variance -> 0")
}
