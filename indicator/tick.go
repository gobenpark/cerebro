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
	"time"
)

type Spread string

const (
	Bid Spread = "bid"
	Ask Spread = "ask"
)

type Tick struct {
	Date      time.Time         `json:"date"`
	Code      string            `json:"code"`
	AskBid    Spread            `json:"askBid"`
	DiffRate  float64           `json:"diffRate"`
	Price     int64             `json:"price"`
	AccVolume int64             `json:"accVolume"`
	Volume    int64             `json:"volume"`
	Metadata  map[string]string `json:"metadata"`
}

type Ticks []Tick

func (t Ticks) Len() int {
	return len(t)
}

func (t Ticks) Less(i, j int) bool {
	return t[i].Date.Before(t[j].Date)
}

func (t Ticks) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t Ticks) Mean() float64 {
	if t.Len() == 0 {
		return 0
	}
	var sum float64
	for _, v := range t {
		sum += float64(v.Price)
	}

	return sum / float64(t.Len())
}

func (t Ticks) StandardDeviation() float64 {
	mean := t.Mean()
	total := 0.0
	for i := range t {
		diff := float64(t[i].Price) - mean
		total += math.Pow(diff, 2)
	}
	variance := total / float64(t.Len()-1)
	return math.Sqrt(variance)
}
