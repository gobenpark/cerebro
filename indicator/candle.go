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
	"time"

	"github.com/shopspring/decimal"
)

type CandleType int

const (
	Min CandleType = iota + 1
	Min3
	Min5
	Min15
	Min60
	Day
)

func (c CandleType) Duration() time.Duration {
	switch c {
	case Min:
		return time.Minute
	case Min3:
		return 3 * time.Minute
	case Min5:
		return 5 * time.Minute
	case Min15:
		return 15 * time.Minute
	case Min60:
		return time.Hour
	case Day:
		return 24 * time.Hour
	default:
		return 0
	}
}

type Candle struct {
	Date          time.Time       `json:"date"`
	Code          string          `json:"code"`
	Type          CandleType      `gorm:"-"`
	Open          decimal.Decimal `json:"open"`
	High          decimal.Decimal `json:"high"`
	Low           decimal.Decimal `json:"low"`
	Close         decimal.Decimal `json:"close"`
	Volume        int64           `json:"volume"`
	Amount        int64           `json:"amount"`
	IndicateValue int64           `json:"indicateValue"`
}
