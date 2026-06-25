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

// Screener is the single source of the watchlist Cerebro trades. Screen streams
// watchlist snapshots until ctx is canceled: each emitted value is the FULL set of
// items to trade at that moment (a declarative snapshot, not a delta), and Cerebro
// reconciles the running per-item strategies (WithStrategyForEach) to match —
// spawning one for a newly added item and, per the EvictionPolicy, retiring one whose
// item dropped out.
//
// A streaming source (e.g. a websocket "top by turnover" feed with the user's filter
// applied) drives dynamic, real-time screening; StaticScreener wraps a fixed list for
// backtests or a known universe. Close the channel when no more snapshots will come
// (a static screen, or a feed that ends); a live source may leave it open until ctx
// is canceled. Cerebro stops reading on shutdown regardless.
type Screener interface {
	Screen(ctx context.Context) <-chan []*item.Item
}

// StaticScreener adapts a fixed list of items to the streaming Screener model: it
// emits the items once and closes, so reconcile spawns a strategy per item and then
// has nothing more to do. Use it for backtests or a universe known up front.
func StaticScreener(items ...*item.Item) Screener {
	return staticScreener(items)
}

type staticScreener []*item.Item

func (s staticScreener) Screen(_ context.Context) <-chan []*item.Item {
	out := make(chan []*item.Item, 1)
	out <- []*item.Item(s)
	close(out)
	return out
}
