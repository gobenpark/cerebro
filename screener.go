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

	"github.com/gobenpark/cerebro/item"
)

// Screener is the source of a dynamic watchlist. Screen streams watchlist snapshots
// until ctx is canceled: each emitted value is the FULL set of items to trade at that
// moment (a declarative snapshot, not a delta), and the reconciler converges the
// running per-item strategies (WithScreener's factory) to match — spawning one for a
// newly added item and, per the EvictionPolicy, retiring one whose item dropped out.
//
// A streaming source (e.g. a websocket "top by turnover" feed with the user's filter
// applied) drives dynamic, real-time screening. Close the channel when no more
// snapshots will come (a feed that ends, or a one-shot screen that emits a single
// fixed list); a live source may leave it open until ctx is canceled. Cerebro stops
// reading on shutdown regardless. A fixed, known universe is usually better expressed
// with WithStrategy(s, codes...) than a one-shot screener.
type Screener interface {
	Screen(ctx context.Context) <-chan []*item.Item
}
