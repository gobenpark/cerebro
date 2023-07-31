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
	"github.com/gobenpark/cerebro/container"
)

func BollingerBand(period int, candles container.Candles) (mid []Indicate, top []Indicate, bottom []Indicate) {

	mid = make([]Indicate, candles.Len())
	top = make([]Indicate, candles.Len())
	bottom = make([]Indicate, candles.Len())
	candleLength := candles.Len()
	queue := container.Candles{}

	for i := range candles {
		if i < period {
			queue = append(queue, candles[i])
			indicate := Indicate{
				Data: 0,
				Date: candles[i].Date,
			}
			mid[i], top[i], bottom[i] = indicate, indicate, indicate
			continue
		}

	}

	slide := candleLength - period
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
	return
}
