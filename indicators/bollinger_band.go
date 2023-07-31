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
	candleLength := candles.Len()
	mid = make([]Indicate, candleLength)
	top = make([]Indicate, candleLength)
	bottom = make([]Indicate, candleLength)

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
		} else {
			queue = append(queue, candles[i])
			mean := queue.Mean()
			sd := queue.StandardDeviation()
			mid[i], top[i], bottom[i] = Indicate{
				Data: 0,
				Date: candles[i].Date,
			}, Indicate{
				Data: mean + (sd * 2),
				Date: candles[i].Date,
			}, Indicate{
				Data: mean - (sd * 2),
				Date: candles[i].Date,
			}
			queue = queue[1:]
		}
	}
	return
}
