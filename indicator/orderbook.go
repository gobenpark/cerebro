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

// Level is one price level of an order book: the resting Size available at Price.
type Level struct {
	Price decimal.Decimal `json:"price"`
	Size  decimal.Decimal `json:"size"`
}

// OrderBook is a snapshot of an instrument's resting limit orders (호가) at a point
// in time. Each side is ordered best-first: Bids[0] is the highest buy price, Asks[0]
// the lowest sell price. Depth varies by venue (KRX publishes 10 levels per side,
// some crypto venues 15–30), so the slices are variable length; either may be empty
// when a side is not quoted.
type OrderBook struct {
	Code string    `json:"code"`
	Date time.Time `json:"date"`
	Bids []Level   `json:"bids"`
	Asks []Level   `json:"asks"`
}

// BestBid returns the highest bid level, or ok=false when the bid side is empty.
func (o OrderBook) BestBid() (Level, bool) {
	if len(o.Bids) == 0 {
		return Level{}, false
	}
	return o.Bids[0], true
}

// BestAsk returns the lowest ask level, or ok=false when the ask side is empty.
func (o OrderBook) BestAsk() (Level, bool) {
	if len(o.Asks) == 0 {
		return Level{}, false
	}
	return o.Asks[0], true
}

// Spread is the best ask price minus the best bid price. ok=false when either side
// is empty, so the spread is undefined (rather than a misleading zero).
func (o OrderBook) Spread() (decimal.Decimal, bool) {
	bid, okB := o.BestBid()
	ask, okA := o.BestAsk()
	if !okB || !okA {
		return decimal.Zero, false
	}
	return ask.Price.Sub(bid.Price), true
}

// Mid is the mid price ((best bid + best ask) / 2). ok=false when either side is
// empty, so the mid is undefined.
func (o OrderBook) Mid() (decimal.Decimal, bool) {
	bid, okB := o.BestBid()
	ask, okA := o.BestAsk()
	if !okB || !okA {
		return decimal.Zero, false
	}
	return bid.Price.Add(ask.Price).Div(decimal.NewFromInt(2)), true
}

// Imbalance is the order-book imbalance over the top n levels per side:
// (bidSize - askSize) / (bidSize + askSize), in [-1, 1]. It is +1 when only bids
// rest and -1 when only asks, signaling buy/sell pressure. n <= 0 uses the full
// depth. ok=false when both sides are empty over the depth (total size zero), so the
// ratio is undefined.
func (o OrderBook) Imbalance(n int) (decimal.Decimal, bool) {
	bidSize := sumSize(o.Bids, n)
	askSize := sumSize(o.Asks, n)
	total := bidSize.Add(askSize)
	if total.IsZero() {
		return decimal.Zero, false
	}
	return bidSize.Sub(askSize).Div(total), true
}

// sumSize totals the size of the first n levels (the full slice when n <= 0 or n
// exceeds the depth).
func sumSize(levels []Level, n int) decimal.Decimal {
	if n <= 0 || n > len(levels) {
		n = len(levels)
	}
	sum := decimal.Zero
	for i := 0; i < n; i++ {
		sum = sum.Add(levels[i].Size)
	}
	return sum
}
