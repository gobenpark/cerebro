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
package indicator

import (
	"time"

	"github.com/shopspring/decimal"
)

type CandleType int

const (
	Min CandleType = iota + 1
	Min3
	Min5
	Min15
	Min60
	Day
)

func (c CandleType) Duration() time.Duration {
	switch c {
	case Min:
		return time.Minute
	case Min3:
		return 3 * time.Minute
	case Min5:
		return 5 * time.Minute
	case Min15:
		return 15 * time.Minute
	case Min60:
		return time.Hour
	case Day:
		return 24 * time.Hour
	default:
		return 0
	}
}

type Candle struct {
	Date          time.Time       `json:"date"`
	Code          string          `json:"code"`
	Type          CandleType      `gorm:"-"`
	Open          decimal.Decimal `json:"open"`
	High          decimal.Decimal `json:"high"`
	Low           decimal.Decimal `json:"low"`
	Close         decimal.Decimal `json:"close"`
	Volume        int64           `json:"volume"`
	Amount        int64           `json:"amount"`
	IndicateValue int64           `json:"indicateValue"`
}

// --- candle shape helpers ---
//
// These describe a single candle's geometry: body, wicks, and direction. Wick
// and body ratios are expressed as a fraction of the full high-low range, so a
// caller can write rules like "upper wick no longer than 20% of the range"
// (c.UpperWickRatio() <= 0.2) without recomputing the parts.
//
// They assume a well-formed candle (High >= Open, Close, Low and Low <= Open,
// Close), which the Resampler and exchange feeds guarantee. A malformed candle
// (e.g. High < Low) yields non-physical negative ranges/wicks rather than an
// error — the caller owns that precondition.

// IsBull reports whether the candle closed above its open (양봉).
func (c *Candle) IsBull() bool { return c.Close.GreaterThan(c.Open) }

// IsBear reports whether the candle closed below its open (음봉).
func (c *Candle) IsBear() bool { return c.Close.LessThan(c.Open) }

// IsDoji reports whether the candle opened and closed at the same price.
func (c *Candle) IsDoji() bool { return c.Close.Equal(c.Open) }

// Range is the full high-low span of the candle.
func (c *Candle) Range() decimal.Decimal { return c.High.Sub(c.Low) }

// Body is the absolute distance between the open and close.
func (c *Candle) Body() decimal.Decimal { return c.Close.Sub(c.Open).Abs() }

// UpperWick is the distance from the body top (max of open/close) to the high.
func (c *Candle) UpperWick() decimal.Decimal {
	return c.High.Sub(decimal.Max(c.Open, c.Close))
}

// LowerWick is the distance from the low to the body bottom (min of open/close).
func (c *Candle) LowerWick() decimal.Decimal {
	return decimal.Min(c.Open, c.Close).Sub(c.Low)
}

// BodyRatio is the body as a fraction of the full range (0 when range is zero).
func (c *Candle) BodyRatio() float64 { return shapeRatio(c.Body(), c.Range()) }

// UpperWickRatio is the upper wick as a fraction of the full range.
func (c *Candle) UpperWickRatio() float64 { return shapeRatio(c.UpperWick(), c.Range()) }

// LowerWickRatio is the lower wick as a fraction of the full range.
func (c *Candle) LowerWickRatio() float64 { return shapeRatio(c.LowerWick(), c.Range()) }

// shapeRatio returns num/den as a float64, or 0 when den is zero (a flat candle
// where high == low, e.g. a single-tick or limit-locked bar).
func shapeRatio(num, den decimal.Decimal) float64 {
	if den.IsZero() {
		return 0
	}
	return num.Div(den).InexactFloat64()
}
