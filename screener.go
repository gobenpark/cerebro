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

// Screener selects the watchlist Cerebro trades. It is the dynamic counterpart of
// WithTargetItem: at Start, Cerebro calls Screen and merges the returned items
// into the target set, so WithStrategyForEach spawns a strategy per selected
// item. Returning items from a screener is how "what to trade" (screening)
// connects to "when to trade" (the strategy).
//
// Screen runs once at Start today; the same interface is the seam for periodic
// re-screening (dynamic spawn/stop of per-item strategies) later.
type Screener interface {
	Screen(ctx context.Context) ([]*item.Item, error)
}
