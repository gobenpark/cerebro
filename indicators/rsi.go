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
package indicators

import "fmt"

type rsiIndicator struct {
	period int
}

func NewRsiIndicator(period int) *rsiIndicator {
	if period == 0 {
		period = 10
	}

	return &rsiIndicator{period: period}
}

// //// self.line[0] = self.line[-1] * self.alpha1 + self.data[0] * self.alpha
func (r *rsiIndicator) Calculate(candles Candles) {
	if candles.Len() < r.period {
		return
	}

	gains := make([]float64, candles.Len())
	losses := make([]float64, candles.Len())

	for i := 1; i < candles.Len(); i++ {
		diff := candles[i].Close - candles[i-1].Close
		if diff > 0 {
			gains[i] = float64(diff)
			losses[i] = 0
		} else {
			losses[i] = float64(-diff)
			gains[i] = 0
		}
	}

	meanGains := Rma(r.period, gains)
	meanLosses := Rma(r.period, losses)

	rsi := make([]float64, candles.Len())

	for i := 0; i < len(rsi); i++ {
		rsi[i] = 100 - (100 / (1 + meanGains[i]/meanLosses[i]))
	}

	for _, i := range rsi {
		fmt.Println(i)
	}

}

func Rma(period int, values []float64) []float64 {
	result := make([]float64, len(values))
	sum := float64(0)

	for i, value := range values {
		count := i + 1

		if i < period {
			sum += value
		} else {
			sum = (result[i-1] * float64(period-1)) + value
			count = period
		}

		result[i] = sum / float64(count)
	}

	return result
}
