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
	"github.com/gobenpark/cerebro/market"
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
	// Extras is the merged pass-through stream of adapter-specific events the core
	// Tick/OrderBook model doesn't cover — an LS adapter's program-trade flow, say.
	// Cerebro routes every event a market emits on Events that it does not recognize as
	// a tick, order-book, or order/balance/feed-status update here, tagged to a code
	// when the event implements Coded (otherwise broadcast to all). It is
	// market-agnostic by design: the core never inspects the value, so a strategy
	// consumes it type-safely via Stream[T] rather than this raw channel. Carries values
	// only when the adapter publishes such events; best-effort delivery (drop-if-behind)
	// like Ticks.
	//
	// SINGLE CONSUMER: read a universe's Extras once — either one Stream[T] (a single
	// adapter type) or one raw range with a type switch (several types). Two readers
	// compete for the one channel and drop each other's values; see Stream.
	Extras() <-chan any
	// Warmup returns a warm candle stream for code at the given level: pre-seeded with
	// the adapter's historical candles (Market.Candles) so indicators are valid from
	// the first bar instead of cold-starting, then advanced as the universe's live
	// ticks close new bars. Call it once per (code, level) at the top of Run; for a
	// single-instrument strategy that code is u.Items()[0].Code.
	//
	// SINGLE CONSUMER of the tick feed: a strategy that uses Warmup must drive its
	// decisions from the returned stream(s), NOT Ticks(). An internal dispatcher
	// consumes the universe's ticks to fold them into candles, so a strategy that also
	// ranges Ticks() competes for the same feed and both drop values. Use one or the
	// other.
	Warmup(ctx context.Context, code string, level market.CandleType) (CandleStream, error)
}

// Coded is an optional capability an adapter-specific Extras event may implement so
// Cerebro fans it out only to the universes subscribed to that code. An event that
// does not implement it is broadcast to every universe's Extras stream, leaving the
// per-code filtering to the strategy.
type Coded interface {
	Code() string
}

// Stream adapts a Universe's untyped Extras channel into a typed <-chan T: a goroutine
// forwards only the Extras values whose dynamic type is T and drops the rest, so a
// strategy consumes an adapter-specific feed with no per-message type assertions — the
// same range pattern as u.Ticks(), but for market-specific data:
//
//	for pf := range strategy.Stream[ls.ProgramTick](ctx, u) { ... }
//
// The goroutine stops (closing the returned channel) when ctx is canceled or Extras
// closes; pass the same ctx the strategy's Run received so a RemoveRunner teardown
// stops it.
//
// CONSTRAINT — a universe's Extras has a SINGLE consumer. Extras is one shared channel,
// so two Stream goroutines on the same universe compete for it: each drops the values
// whose dynamic type isn't its own T, so a value taken by the wrong stream is lost
// nondeterministically. Use Stream for the one-adapter-type case. A strategy that needs
// MORE THAN ONE adapter-specific type must NOT call Stream twice — range u.Extras()
// directly and type-switch instead:
//
//	for e := range u.Extras() {
//	    switch ev := e.(type) {
//	    case ls.ProgramTick:    // ...
//	    case ls.SomeOtherEvent: // ...
//	    }
//	}
func Stream[T any](ctx context.Context, u Universe) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		in := u.Extras()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-in:
				if !ok {
					return
				}
				v, vok := e.(T)
				if !vok {
					continue
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
