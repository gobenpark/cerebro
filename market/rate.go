/*
 *  Copyright 2024 The Cerebro Authors
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
package market

import "github.com/shopspring/decimal"

// Rate is a commission rate held internally as a fraction of trade value
// (0.0015 == 0.15%). Build it through Percent or Fraction so the unit is
// explicit at the call site — the broker charges it with Of and never has to
// assume whether a bare decimal meant a percentage or a fraction, which is how a
// 100x fee mistake used to slip in. The zero value is a zero (free) commission.
type Rate struct {
	frac decimal.Decimal
}

// hundred converts a percentage to its fraction; defined here so the magic
// number lives next to the unit contract rather than scattered in the broker.
var hundred = decimal.NewFromInt(100)

// Percent builds a Rate from a percentage: Percent(0.15) is 0.15%, a 0.0015
// fraction of trade value.
func Percent(pct decimal.Decimal) Rate {
	return Rate{frac: pct.Div(hundred)}
}

// Fraction builds a Rate from a fraction of trade value directly:
// Fraction(0.0015) is 0.15%.
func Fraction(frac decimal.Decimal) Rate {
	return Rate{frac: frac}
}

// Of returns the commission charged on a trade value (value * fraction).
func (r Rate) Of(value decimal.Decimal) decimal.Decimal {
	return value.Mul(r.frac)
}

// AsFraction returns the underlying fraction of trade value, for logging or
// reporting the effective rate.
func (r Rate) AsFraction() decimal.Decimal {
	return r.frac
}

// IsZero reports whether the rate charges no commission.
func (r Rate) IsZero() bool {
	return r.frac.IsZero()
}
