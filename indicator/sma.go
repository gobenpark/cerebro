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

//func SMA(c)

//
//type sma struct {
//	period    int
//	indicates []Indicate
//	limit     int
//}
//
//func NewSMA(period int, limit int) Indicator {
//	return &sma{period: period, limit: limit}
//}
//
//func (s *sma) Calculate(candles candles) {
//
//	U := []int{}
//	D := []int{}
//
//	for i := 0; i < candles.Len()-1; i++ {
//		diff := candles[i+1].Close - candles[i].Close
//		if diff > 0 {
//			U = append(U, candles[i+1].Close-candles[i].Close)
//			D = append(D, 0)
//		} else {
//			U = append(U, 0)
//			D = append(D, candles[i+1].Close-candles[i].Close)
//		}
//	}
//
//	size := candles.Len()
//	if size >= s.period {
//		slide := (size - s.period)
//
//		for i := 0; i <= slide; i++ {
//			id := Indicate{
//				Data: average(candles[i : s.period+i]),
//				Date: candles[(s.period+i)-1].Date,
//			}
//
//			if len(s.indicates) != 0 {
//				if id.Date.After(s.indicates[0].Date) {
//					indicates = append(indicates, id)
//					continue
//				}
//				break
//			} else {
//				indicates = append(indicates, id)
//			}
//		}
//		s.indicates = append(indicates, s.indicates...)
//	}
//}
