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
package cerebro

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// Eviction describes a per-item strategy the screener has dropped from the
// watchlist, handed to an EvictionPolicy to decide its fate.
type Eviction struct {
	Strategy string            // the dropped strategy's Name()
	Item     *item.Item        // the item it traded
	Position position.Position // its current position (zero size when flat)
	// Pending is true when the strategy still has an open order for the item — an
	// entry that has not filled yet, or an exit in flight. A policy must treat a
	// pending strategy as not-yet-settled even when Position is flat, or a fill landing
	// after eviction would leave a position no strategy manages.
	Pending bool
}

// EvictionPolicy decides what happens to a per-item strategy when the screener drops
// its item from the watchlist. It returns true to tear the runner down now, or false
// to keep it for re-evaluation on the next screen (e.g. until its position is flat).
// The submitter is scoped to the strategy, so a policy may place exit orders before
// returning. It is consulted on the reconcile goroutine, one item at a time, and may
// be called across several screens for the same item until it returns true.
type EvictionPolicy func(ctx context.Context, e Eviction, sub broker.Submitter) (evict bool)

// KeepUntilFlat tears a dropped strategy down only once it holds no position AND has
// no order pending, so the screener never orphans or force-closes a position — the
// dropped strategy (and its reactive exit policy) keeps running until it is settled.
// It is the default.
func KeepUntilFlat(_ context.Context, e Eviction, _ broker.Submitter) bool {
	return !e.Pending && e.Position.Size.LessThanOrEqual(decimal.Zero)
}

// Flatten submits a market exit for any open position, then keeps the strategy until
// a later screen finds it settled (flat and no order pending) and tears it down. Use
// it to actively close out a position the moment the screener drops the item.
func Flatten(ctx context.Context, e Eviction, sub broker.Submitter) bool {
	if e.Pending {
		// An order is already in flight — an unfilled entry, or the exit submitted on a
		// previous screen. Wait for it to settle rather than stacking another exit.
		return false
	}
	if e.Position.Size.GreaterThan(decimal.Zero) {
		_ = sub.Order(ctx, order.NewOrder(e.Item, order.Sell, order.Market, e.Position.Size, decimal.Zero), false)
		return false
	}
	return true // flat and settled
}

// DropImmediately tears the strategy down at once, leaving any open position
// unmanaged. Use only when something outside Cerebro manages exits.
func DropImmediately(_ context.Context, _ Eviction, _ broker.Submitter) bool { return true }
