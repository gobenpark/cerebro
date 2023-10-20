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

// import (
// 	"github.com/gobenpark/cerebro/container"
// )

// func BollingerBand(period int, candles container.Candles) (mid []Indicate, top []Indicate, bottom []Indicate) {
// 	candleLength := candles.Len()
// 	if candleLength < period {
// 		return
// 	}
// 	mid = make([]Indicate, candleLength-period)
// 	top = make([]Indicate, candleLength-period)
// 	bottom = make([]Indicate, candleLength-period)

// 	for i := 0; i < candleLength-period; i++ {
// 		mean := candles[i : i+period].Mean()
// 		sd := candles[i : i+period].StandardDeviation()
// 		mid[i], top[i], bottom[i] = Indicate{
// 			Data: mean,
// 			Date: candles[i+period].Date,
// 		}, Indicate{
// 			Data: mean + (sd * 2),
// 			Date: candles[i+period].Date,
// 		}, Indicate{
// 			Data: mean - (sd * 2),
// 			Date: candles[i+period].Date,
// 		}
// 	}
// 	return
// }
