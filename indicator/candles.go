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

func (c Candles) SMA(period int) Candles {

	U := []int64{}
	D := []int64{}

	for i := 0; i < c.Len()-1; i++ {
		diff := c[i+1].Close - c[i].Close
		if diff > 0 {
			U = append(U, c[i+1].Close-c[i].Close)
			D = append(D, 0)
		} else {
			U = append(U, 0)
			D = append(D, c[i+1].Close-c[i].Close)
		}
	}

	size := c.Len()
	if size >= period {
		slide := (size - period)

		for i := 0; i <= slide; i++ {
			//
			//	id := Indicate{
			//		Data: average(candles[i : s.period+i]),
			//		Date: candles[(s.period+i)-1].Date,
			//	}
			//
			//	if len(s.indicates) != 0 {
			//		if id.Date.After(s.indicates[0].Date) {
			//			indicates = append(indicates, id)
			//			continue
			//		}
			//		break
			//	} else {
			//		indicates = append(indicates, id)
			//	}
			//}
			//s.indicates = append(indicates, s.indicates...)
		}
	}
	return nil
}
