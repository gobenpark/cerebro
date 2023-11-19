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
)

type Candles []Candle

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
