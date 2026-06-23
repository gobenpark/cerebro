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
package strategy

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

import (
	"context"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

type CandleType int

type Strategy interface {
	// Run receives ticks for its Universe until ctx is canceled. Implementations
	// must return when ctx.Done() fires so the engine can shut down cleanly. The
	// broker handle is scoped to this strategy: orders it submits are attributed to
	// Name().
	//
	// A single-instrument strategy reads u.Items()[0] and ranges over u.Ticks(); a
	// pairs/portfolio strategy ranges over u.Items() and demultiplexes u.Ticks() by
	// indicator.Tick.Code.
	Run(ctx context.Context, u Universe, b broker.Submitter)
	// NotifyOrder is when event rise order then called
	NotifyOrder(o order.Order)
	NotifyTrade()
	NotifyFund()
	Name() string
}

// Universe is the set of instruments a strategy trades together, plus their
// merged realtime tick stream. It is the unit a strategy decides over: one
// instrument for a plain strategy, several for a pairs/portfolio strategy. Ticks
// from every item in the universe arrive on the single Ticks() channel, tagged by
// indicator.Tick.Code.
type Universe interface {
	Items() []*item.Item
	Ticks() <-chan indicator.Tick
}
