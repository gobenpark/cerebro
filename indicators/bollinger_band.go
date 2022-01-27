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
package indicators

import (
	"math"

	"github.com/gobenpark/trader/container"
)

type BollingerBand struct {
	period int
	Top    []Indicate
	Mid    []Indicate
	Bottom []Indicate
}

func NewBollingerBand(period int) *BollingerBand {
	return &BollingerBand{period: period}
}

func (b *BollingerBand) mean(data []container.Candle) float64 {
	total := 0.0
	for _, i := range data {
		total += i.Close
	}

	return total / float64(len(data))
}

func (b *BollingerBand) standardDeviation(mean float64, data []container.Candle) float64 {
	total := 0.0
	for _, i := range data {
		da := i.Close - mean
		total += math.Pow(da, 2)
	}
	return math.Sqrt(total / float64(len(data)))
}

func (b *BollingerBand) Calculate(candles []container.Candle) {

	if len(candles) < b.period {
		return
	}

	slice := len(candles) - b.period
	for i := slice - 1; i >= 0; i-- {
		mean := b.mean(candles[i : i+b.period])
		sd := b.standardDeviation(mean, candles[i:i+b.period])

		b.Mid = append([]Indicate{{
			Data: mean,
			Date: candles[i].Date,
		}}, b.Mid...)

		b.Top = append([]Indicate{{
			Data: mean + (sd * 2),
			Date: candles[i].Date,
		}}, b.Top...)

		b.Bottom = append([]Indicate{{
			Data: mean - (sd * 2),
			Date: candles[i].Date,
		}}, b.Bottom...)
	}

}

func (b *BollingerBand) Get() []Indicate {
	panic("implement me")
}
