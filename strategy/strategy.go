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
	"github.com/gobenpark/cerebro/risk"
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

// RiskAware is an optional capability a Strategy may implement to declare its own
// reactive exit policy (stop-loss / trailing-stop / take-profit). Cerebro registers
// the returned Policy with the risk Monitor under the strategy's Name(), so a
// strategy and its exit rule travel as one unit. This is the path that reaches
// dynamically spawned strategies (WithStrategyForEach), where a name-based
// WithRiskPolicy cannot — the name isn't known until spawn. A strategy that does
// not implement it, or returns a disabled Policy, has no reactive exit. An explicit
// WithRiskPolicy for the same Name() overrides the declared policy; a disabled
// (empty) one clears it, so a caller can turn a built-in ExitPolicy off by name.
type RiskAware interface {
	Strategy
	ExitPolicy() risk.Policy
}

// Universe is the set of instruments a strategy trades together, plus their
// merged realtime tick stream. It is the unit a strategy decides over: one
// instrument for a plain strategy, several for a pairs/portfolio strategy. Ticks
// from every item in the universe arrive on the single Ticks() channel, tagged by
// indicator.Tick.Code.
type Universe interface {
	Items() []*item.Item
	Ticks() <-chan indicator.Tick
	// OrderBooks is the merged order-book (호가) stream for the universe, tagged by
	// indicator.OrderBook.Code. It carries values only when the market adapter
	// publishes order books; a strategy that decides on trades alone can ignore it.
	// Like Ticks, updates are delivered best-effort — a strategy that does not keep up
	// drops snapshots rather than stalling the feed.
	OrderBooks() <-chan indicator.OrderBook
}
