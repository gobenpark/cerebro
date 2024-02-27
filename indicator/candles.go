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
	total := int64(0)
	for i := range c {
		total += c[i].Close
	}
	return float64(total) / float64(c.Len())
}

func (c Candles) StandardDeviation() float64 {
	mean := c.Mean()
	total := 0.0
	for i := range c {
		diff := float64(c[i].Close) - mean
		total += math.Pow(diff, 2)
	}
	variance := total / float64(c.Len()-1)
	return math.Sqrt(variance)
}

func (c Candles) MACD(fast, slow, signal int) (macdLine []Indicate[float64], signalLine []Indicate[float64]) {
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
		return float64(item.Close)
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

// calculate bollinger band with candle
// period is the number of candles to calculate the mean and standard deviation of the candle
// period must be greater than 1 and day
func (c Candles) BollingerBand(period int) (bottom []Indicate[float64], mid []Indicate[float64], top []Indicate[float64]) {
	candleLength := c.Len()
	if candleLength < period {
		return
	}
	mid = make([]Indicate[float64], candleLength)
	top = make([]Indicate[float64], candleLength)
	bottom = make([]Indicate[float64], candleLength)

	for i := 0; i < candleLength; i++ {
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
			case cds[i-1].Close < cds[i].Close:
				up += float64(cds[i].Volume)
			case cds[i-1].Close > cds[i].Close:
				down += float64(cds[i].Volume)
			case cds[i-1].Close == cds[i].Close:
				up += float64(cds[i].Volume) / 2
				down += float64(cds[i].Volume) / 2
			}
		}
		return (up / down) * 100.0
	}

	value := make([]Indicate[float64], c.Len())
	candleLength := c.Len()
	if candleLength < nday {
		return nil
	}

	for i := 0; i < candleLength; i++ {
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
	faskK, _ := c.StochasticFast(k, d, period)

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

func (c Candles) StochasticFast(k, d, period int) (K, D []Indicate[float64]) {
	if k == 0 {
		k = 3
	}

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

		high := c[i-period].Close
		low := c[i-period].Close

		for j := range c[i-period : i+1] {
			if high < c[i-period : i+1][j].High {
				high = c[i-period : i+1][j].High
			}

			if low > c[i-period : i+1][j].Low {
				low = c[i-period : i+1][j].Low
			}
		}
		K[i] = Indicate[float64]{
			Data: ((float64(c[i].Close) - float64(low)) / (float64(high) - float64(low))) * 100,
			Date: c[i].Date,
		}
	}

	for i := range K {
		if i < k-1 {
			D[i] = Indicate[float64]{
				Data: 0,
				Date: c[i].Date,
			}
			continue
		}

		data := lo.Map[Indicate[float64]](K[i-k+1:i+1], func(item Indicate[float64], index int) float64 {
			return item.Data
		})

		D[i] = Indicate[float64]{
			Data: lo.Sum(data) / float64(k),
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
			Data: mean - (mean * up),
			Date: c[i].Date,
		}
	}
	return
}
