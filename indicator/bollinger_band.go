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

func BollingerBand(period int, candles Candles) (mid []Indicate[float64], top []Indicate[float64], bottom []Indicate[float64]) {
	candleLength := candles.Len()
	if candleLength < period {
		return
	}
	mid = make([]Indicate[float64], candleLength-period)
	top = make([]Indicate[float64], candleLength-period)
	bottom = make([]Indicate[float64], candleLength-period)

	for i := 0; i < candleLength-period; i++ {
		mean := candles[i : i+period].Mean()
		sd := candles[i : i+period].StandardDeviation()

		mid[i], top[i], bottom[i] = Indicate[float64]{
			Data: mean,
			Date: candles[i+period].Date,
		}, Indicate[float64]{
			Data: mean + (sd * 2),
			Date: candles[i+period].Date,
		}, Indicate[float64]{
			Data: mean - (sd * 2),
			Date: candles[i+period].Date,
		}
	}
	return
}
