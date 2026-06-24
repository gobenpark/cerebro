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
	"math"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type Candles []*Candle

func (c Candles) Len() int {
	return len(c)
}

func (c Candles) Less(i, j int) bool {
	return c[i].Date.Before(c[j].Date)
}

func (c Candles) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c Candles) Mean() float64 {
	if c.Len() == 0 {
		return 0 // no samples: mean is undefined, avoid 0/0 = NaN
	}
	total := 0.0
	for i := range c {
		total += c[i].Close.InexactFloat64()
	}
	return total / float64(c.Len())
}

func (c Candles) StandardDeviation() float64 {
	if c.Len() < 2 {
		return 0 // sample variance needs >= 2 points (the n-1 denominator)
	}
	mean := c.Mean()
	total := 0.0
	for i := range c {
		diff := c[i].Close.InexactFloat64() - mean
		total += diff * diff
	}
	variance := total / float64(c.Len()-1)
	return math.Sqrt(variance)
}

func (c Candles) MACD(fast, slow, signal int) (macdLine, signalLine []Indicate[float64]) {
	if fast == 0 {
		fast = 12
	}

	if slow == 0 {
		slow = 26
	}

	if signal == 0 {
		signal = 9
	}

	candleLength := c.Len()
	if candleLength < slow {
		return
	}

	ema := func(data []float64, period int) []float64 {
		result := make([]float64, len(data))
		k := 2.0 / float64(period+1)

		result[0] = data[0]
		for i := 1; i < len(data); i++ {
			result[i] = k*data[i] + (1-k)*result[i-1]
		}

		return result
	}

	cds := lo.Map[*Candle](c, func(item *Candle, index int) float64 {
		return item.Close.InexactFloat64()
	})

	f := ema(cds, fast)
	s := ema(cds, slow)
	macd := make([]float64, c.Len())
	for i := range f {
		macd[i] = f[i] - s[i]
	}

	signals := ema(macd, signal)
	macdLine = make([]Indicate[float64], c.Len())
	signalLine = make([]Indicate[float64], c.Len())

	for i := range c {
		macdLine[i], signalLine[i] = Indicate[float64]{
			Data: macd[i],
			Date: c[i].Date,
		}, Indicate[float64]{
			Data: signals[i],
			Date: c[i].Date,
		}
	}

	return
}

// BollingerBand calculates the bollinger band over period candles, returning the
// bottom, middle, and top bands. period must be greater than 1; a non-positive
// period falls back to the conventional 20, matching the other indicators here so
// a zero-value argument never collapses the window to a single point (which would
// make the standard deviation undefined).
func (c Candles) BollingerBand(period int) (bottom, mid, top []Indicate[float64]) {
	if period <= 0 {
		period = 20
	}
	candleLength := c.Len()
	if candleLength < period {
		return
	}
	mid = make([]Indicate[float64], candleLength)
	top = make([]Indicate[float64], candleLength)
	bottom = make([]Indicate[float64], candleLength)

	for i := range candleLength {
		if i < period {
			bottom[i], mid[i], top[i] = Indicate[float64]{}, Indicate[float64]{}, Indicate[float64]{}
			continue
		}
		mean := c[i-period : i+1].Mean()
		sd := c[i-period : i+1].StandardDeviation()
		mid[i], top[i], bottom[i] = Indicate[float64]{
			Data: mean,
			Date: c[i].Date,
		}, Indicate[float64]{
			Data: math.Round(mean + (sd * 2)),
			Date: c[i].Date,
		}, Indicate[float64]{
			Data: math.Round(mean - (sd * 2)),
			Date: c[i].Date,
		}
	}
	return
}

func (c Candles) VolumeRatio(nday int) []Indicate[float64] {

	vr := func(cds Candles) float64 {
		up := 0.0
		down := 0.0
		for i := 1; i < cds.Len(); i++ {
			switch {
			case cds[i-1].Close.LessThan(cds[i].Close):
				up += float64(cds[i].Volume)
			case cds[i-1].Close.GreaterThan(cds[i].Close):
				down += float64(cds[i].Volume)
			default:
				up += float64(cds[i].Volume) / 2
				down += float64(cds[i].Volume) / 2
			}
		}
		if down == 0 {
			// No down-volume in the window (every step up, or fewer than two
			// candles): the ratio is undefined. Report 0 rather than +Inf, matching
			// the 0 this function already uses for not-yet-computed entries.
			return 0
		}
		return (up / down) * 100.0
	}

	value := make([]Indicate[float64], c.Len())
	candleLength := c.Len()
	if candleLength < nday {
		return nil
	}

	for i := range candleLength {
		if i < nday {
			value[i] = Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}
			continue
		}
		value[i] = Indicate[float64]{
			Data: vr(c[i-nday : i+1]),
			Date: c[i].Date,
		}
	}
	return value
}

func (c Candles) StochasticSlow(k, d, period int) (K, D []Indicate[float64]) {
	if k == 0 {
		k = 3
	}

	if d == 0 {
		d = 3
	}

	if period == 0 {
		period = 14
	}

	K = make([]Indicate[float64], c.Len())
	D = make([]Indicate[float64], c.Len())
	faskK, _ := c.StochasticFast(d, period)

	for i := range faskK {
		if i < k-1 {
			K[i] = Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}
			continue
		}

		data := lo.Map[Indicate[float64]](faskK[i-k+1:i+1], func(item Indicate[float64], index int) float64 {
			return item.Data
		})

		K[i] = Indicate[float64]{
			Data: lo.Sum(data) / float64(k),
			Date: faskK[i].Date,
		}
	}

	for i := range K {
		if i < d-1 {
			D[i] = Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}
			continue
		}

		data := lo.Map[Indicate[float64]](K[i-d+1:i+1], func(item Indicate[float64], index int) float64 {
			return item.Data
		})

		D[i] = Indicate[float64]{
			Data: lo.Sum(data) / float64(d),
			Date: K[i].Date,
		}
	}

	return
}

func (c Candles) StochasticFast(d, period int) (K, D []Indicate[float64]) {
	if d == 0 {
		d = 3
	}

	if period == 0 {
		period = 14
	}

	D = make([]Indicate[float64], c.Len())
	K = make([]Indicate[float64], c.Len())

	for i := range c {
		if i < period {
			K[i] = Indicate[float64]{Date: c[i].Date}
			continue
		}

		window := c[i-period : i+1]
		high := c[i-period].Close.InexactFloat64()
		low := c[i-period].Close.InexactFloat64()

		for j := range window {
			high = max(high, window[j].High.InexactFloat64())
			low = min(low, window[j].Low.InexactFloat64())
		}
		// A flat window (high == low) has no range, so %K's position within it is
		// undefined; use the neutral 50 instead of dividing 0/0 into a NaN.
		k := 50.0
		if rng := high - low; rng != 0 {
			k = ((c[i].Close.InexactFloat64() - low) / rng) * 100
		}
		K[i] = Indicate[float64]{
			Data: k,
			Date: c[i].Date,
		}
	}

	for i := range K {
		if i < d-1 {
			D[i] = Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}
			continue
		}

		data := lo.Map(K[i-d+1:i+1], func(item Indicate[float64], index int) float64 {
			return item.Data
		})

		D[i] = Indicate[float64]{
			Data: lo.Sum(data) / float64(d),
			Date: K[i].Date,
		}
	}
	return
}

// Envelope indicator, period (number) up,down percentage
func (c Candles) Envelope(period int, up, down float64) (sma, upper, lower []Indicate[float64]) {

	if period == 0 {
		period = 20
	}

	if up == 0 {
		up = 0.1
	}

	if down == 0 {
		down = 0.1
	}

	sma = make([]Indicate[float64], c.Len())
	upper = make([]Indicate[float64], c.Len())
	lower = make([]Indicate[float64], c.Len())

	for i := range c {
		if i < period-1 {
			sma[i], upper[i], lower[i] = Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}, Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}, Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}
			continue
		}
		mean := c[i-period+1 : i+1].Mean()
		sma[i], upper[i], lower[i] = Indicate[float64]{
			Data: mean,
			Date: c[i].Date,
		}, Indicate[float64]{
			Data: mean + (mean * up),
			Date: c[i].Date,
		}, Indicate[float64]{
			Data: mean - (mean * down),
			Date: c[i].Date,
		}
	}
	return
}

// Highest returns the highest High among the last period candles (or all of them
// when fewer than period exist). It returns zero for an empty series. Useful for
// prior-high / breakout proximity checks (e.g. how close price is to the recent
// high).
func (c Candles) Highest(period int) decimal.Decimal {
	if c.Len() == 0 {
		return decimal.Zero
	}
	// period <= 0 or period >= len means "the whole series". Subtract only when
	// the window is a strict suffix, so a pathological negative period cannot
	// overflow c.Len() - period into a positive (out-of-range) start.
	start := 0
	if period > 0 && period < c.Len() {
		start = c.Len() - period
	}
	h := c[start].High
	for _, cd := range c[start+1:] {
		if cd.High.GreaterThan(h) {
			h = cd.High
		}
	}
	return h
}

// Lowest returns the lowest Low among the last period candles (or all of them
// when fewer than period exist). It returns zero for an empty series.
func (c Candles) Lowest(period int) decimal.Decimal {
	if c.Len() == 0 {
		return decimal.Zero
	}
	// See Highest: keep the subtraction in range so a negative period cannot
	// overflow into an out-of-range start.
	start := 0
	if period > 0 && period < c.Len() {
		start = c.Len() - period
	}
	l := c[start].Low
	for _, cd := range c[start+1:] {
		if cd.Low.LessThan(l) {
			l = cd.Low
		}
	}
	return l
}

// SMA returns the simple moving average of closing prices over period candles.
// Like the other indicators, entries before period-1 are zero-valued so the
// result aligns index-for-index with the candle series. period <= 0 is treated
// as 1.
func (c Candles) SMA(period int) []Indicate[float64] {
	if period <= 0 {
		period = 1
	}
	out := make([]Indicate[float64], c.Len())
	for i := range c {
		if i < period-1 {
			out[i] = Indicate[float64]{Date: c[i].Date}
			continue
		}
		out[i] = Indicate[float64]{
			Data: c[i-period+1 : i+1].Mean(),
			Date: c[i].Date,
		}
	}
	return out
}
