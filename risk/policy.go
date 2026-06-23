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

package risk

import "github.com/shopspring/decimal"

// Policy is a reactive, per-strategy exit policy. Unlike a Rule (which vets an
// order before it is placed), a Policy watches a strategy's open position and
// triggers an exit after the fact. A Monitor evaluates it on every tick.
//
// Each threshold is a fraction of price (e.g. 0.03 == 3%); a zero value disables
// that trigger. Triggers are checked in order of urgency: stop-loss, then
// trailing-stop, then take-profit.
type Policy struct {
	// StopLoss exits when price falls to entry*(1-StopLoss).
	StopLoss float64
	// TrailingStop exits when price falls to peak*(1-TrailingStop), where peak is
	// the highest price seen since entry, so the stop ratchets up with the position.
	TrailingStop float64
	// TakeProfit exits when price rises to entry*(1+TakeProfit).
	TakeProfit float64
}

// Enabled reports whether the policy has at least one trigger configured.
func (p Policy) Enabled() bool {
	return p.StopLoss > 0 || p.TrailingStop > 0 || p.TakeProfit > 0
}

// triggered reports the first exit trigger hit at price for a long position with
// the given average entry and peak-since-entry price, or ok=false if none fire.
// An entry of zero or less cannot be evaluated (the fill price is unknown).
func (p Policy) triggered(entry, peak, price decimal.Decimal) (reason string, ok bool) {
	if entry.LessThanOrEqual(decimal.Zero) {
		return "", false
	}
	one := decimal.NewFromInt(1)
	if p.StopLoss > 0 {
		stop := entry.Mul(one.Sub(decimal.NewFromFloat(p.StopLoss)))
		if price.LessThanOrEqual(stop) {
			return "stop-loss", true
		}
	}
	if p.TrailingStop > 0 && peak.GreaterThan(decimal.Zero) {
		stop := peak.Mul(one.Sub(decimal.NewFromFloat(p.TrailingStop)))
		if price.LessThanOrEqual(stop) {
			return "trailing-stop", true
		}
	}
	if p.TakeProfit > 0 {
		target := entry.Mul(one.Add(decimal.NewFromFloat(p.TakeProfit)))
		if price.GreaterThanOrEqual(target) {
			return "take-profit", true
		}
	}
	return "", false
}
