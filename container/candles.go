/*
 *  Copyright 2021 The Trader Authors
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

package container

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
		da := float64(c[i].Close) - mean
		total += math.Pow(da, 2)
	}
	variance := total / float64(c.Len()-1)
	return math.Sqrt(variance)
}
