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

type rsi struct {
	period    int
	indicates []Indicate
	AD        []Indicate
	AU        []Indicate
}

func NewRsi(period int) Indicator {
	if period == 0 {
		period = 14
	}
	return &rsi{period: period, indicates: []Indicate{}}
}

//self.line[0] = self.line[-1] * self.alpha1 + self.data[0] * self.alpha
func (r *rsi) Calculate(c []container.Candle) {
	if len(c) > 100 {
		c = c[:100]
	}

	slide := len(c) - r.period
	if len(c) < r.period {
		return
	}
	alpha := 1.0 / float64(r.period)
	alpha1 := 1.0 - alpha
	aprev := 0.0
	uprev := 0.0

	if v := c[slide].Close - c[slide+1].Close; v > 0 {
		aprev = v
	} else {
		uprev = math.Abs(v)
	}

	var a []float64
	var b []float64

	for i := slide - 1; i >= 0; i-- {
		if v := c[i].Close - c[i+1].Close; v >= 0 {
			aprev = aprev*alpha1 + v*alpha
			a = append([]float64{aprev}, a...)

			uprev *= alpha1
			b = append([]float64{uprev}, b...)
		} else {
			aprev *= alpha1
			a = append([]float64{aprev}, a...)

			uprev = uprev*alpha1 + math.Abs(v)*alpha
			b = append([]float64{uprev}, b...)
		}

		rs := aprev / uprev
		rsi := 100.0 - 100.0/(1.0+rs)
		r.indicates = append([]Indicate{{
			Data: rsi,
			Date: c[i].Date,
		}}, r.indicates...)
	}
}

func (r *rsi) Get() []Indicate {
	return r.indicates
}

func (r *rsi) PeriodSatisfaction() bool {
	panic("implement me")
}
