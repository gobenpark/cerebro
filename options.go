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
	"log/slog"
	"time"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

type Option func(*Cerebro)

func WithMarket(s market.Market) Option {
	return func(c *Cerebro) {
		c.market = s
	}
}

// WithScreener registers a dynamic screening group: the screener streams watchlist
// snapshots, and for each screened item Cerebro spawns a per-item strategy from
// factory, retiring it (per the eviction policy) when the item drops out. Call it
// again for an independent group — its own screener, factory, and policy — so several
// screeners can drive several strategies at once. For a fixed, known universe prefer
// WithStrategy(s, codes...). The factory must give each instance a unique Name() (e.g.
// derived from the item's Code) so orders attribute and route one-to-one.
func WithScreener(s Screener, factory func(*item.Item) strategy.Strategy, opts ...ScreenOption) Option {
	return func(c *Cerebro) {
		g := screenGroup{screener: s, factory: factory, evict: KeepUntilFlat}
		for _, o := range opts {
			o(&g)
		}
		c.screenGroups = append(c.screenGroups, g)
	}
}

// ScreenOption configures one WithScreener group.
type ScreenOption func(*screenGroup)

// WithEviction sets a screening group's eviction policy — what happens to a per-item
// strategy when the screener drops its code. Defaults to KeepUntilFlat (retires it
// only once flat, never orphaning or force-closing a position); Flatten and
// DropImmediately are also provided, or pass your own.
func WithEviction(p EvictionPolicy) ScreenOption {
	return func(g *screenGroup) {
		g.evict = p
	}
}

// WithLogLevel sets the level of the default stderr logger. It is ignored when a
// logger is supplied with WithLogger.
func WithLogLevel(lvl slog.Level) Option {
	return func(c *Cerebro) {
		c.logLevel = lvl
	}
}

// WithLogger routes Cerebro's logs through the given slog.Logger instead of the
// default stderr handler, so they can join an existing logging pipeline. Pass
// slog.New(slog.DiscardHandler) to silence Cerebro entirely.
func WithLogger(l *slog.Logger) Option {
	return func(c *Cerebro) {
		c.log = l
	}
}

// WithStrategy registers one strategy over an explicit, fixed universe: the codes are
// the instruments it trades together — give two or more for a pairs/portfolio
// strategy that decides over them in one Run. At least one code is required (a
// strategy with no universe has nothing to trade). Call it again to add more
// strategies. Orders are attributed to the strategy's Name(), which must be unique
// across all registrations. For a dynamic, screener-driven universe use WithScreener.
func WithStrategy(s strategy.Strategy, codes ...string) Option {
	return func(c *Cerebro) {
		c.stratRegs = append(c.stratRegs, stratReg{s: s, codes: codes})
	}
}

func WithStrategyTimeout(du time.Duration) Option {
	return func(c *Cerebro) {
		c.timeout = du
	}
}

// WithRisk installs a pre-trade risk gate built from the given rules. Without it
// there is no gate (existing behavior).
func WithRisk(rules ...risk.Rule) Option {
	return func(c *Cerebro) {
		c.risk = risk.New(rules...)
	}
}

// WithStorage installs a durable store for the broker's per-strategy ledger
// (realized PnL, fees, and open lots). On Start the broker restores any persisted
// ledger before processing events, and after each booked fill it writes the
// updated ledger back, so trading state survives a process restart. Without it
// there is no persistence (existing behavior). Cash balance and account
// positions are always re-fetched from the exchange and are not persisted.
func WithStorage(s broker.Storage) Option {
	return func(c *Cerebro) {
		c.storage = s
	}
}

// WithFeedTimeout arms a market-data staleness watchdog: if no tick (or
// market.FeedStatusEvent) arrives within d, the feed is considered stale and the
// feed-loss handler runs (by default a fail-safe Shutdown — see WithFeedLossHandler).
// Zero (the default) disables the watchdog, which suits backtests whose replay
// channel closes cleanly when the data is exhausted. Set it for a live feed to a
// value comfortably above the largest expected gap between ticks plus the adapter's
// reconnect time, so a normal quiet period is not mistaken for a dead feed.
func WithFeedTimeout(d time.Duration) Option {
	return func(c *Cerebro) {
		c.feedTimeout = d
	}
}

// WithFeedLossHandler installs a callback invoked when the market feed is lost —
// either it goes stale (see WithFeedTimeout) or its event channel closes while the
// run is still live. It replaces the default fail-safe, which Shuts the engine down
// so it does not keep trading on a dead feed. Installing a handler also enables
// channel-close-as-loss detection even when no feed timeout is set. The handler runs
// on its own goroutine and may be invoked more than once (e.g. a stale trip followed
// by a channel close), so it must be safe for concurrent use and should be
// idempotent. A handler that reconnects must do so through the market adapter; once
// the channel has closed the engine no longer pumps it.
func WithFeedLossHandler(fn func(reason string)) Option {
	return func(c *Cerebro) {
		c.feedLossHandler = fn
	}
}

// WithOrderTimeout bounds how long the broker waits for the market adapter's Order
// call to return before treating the submission as in-doubt. Zero (the default) leaves
// it unbounded, so only the adapter's own context governs the call (existing behavior).
//
// A timeout is NOT a rejection. When it trips, the order may well have reached the
// exchange — its outcome is simply unknown — so the broker keeps the order open and
// its cash reserved rather than rejecting it (which would free the cash while the
// exchange might still be working the order, risking a double exposure). The order
// stays Submitted until a later exchange fill/cancel event, a restart reconcile
// (OpenOrderReporter), or an operator (WithInDoubtHandler) resolves it. Set it for a
// live adapter; leave it off for backtests, whose simulated Order returns promptly.
func WithOrderTimeout(d time.Duration) Option {
	return func(c *Cerebro) {
		c.orderTimeout = d
	}
}

// WithInDoubtHandler installs a callback invoked when an order submission ends
// in-doubt: the adapter's Order call exceeded WithOrderTimeout, so whether the
// exchange received the order is unknown. The order is left open with its cash still
// reserved; the handler is the operator's hook to resolve it — query the exchange by
// the order's (client) id, alert, or attempt a cancel. It runs on its own goroutine
// and receives a copy of the order plus the underlying error. Without it the broker
// only logs the in-doubt submission. It never fires unless WithOrderTimeout is set.
func WithInDoubtHandler(fn func(o order.Order, err error)) Option {
	return func(c *Cerebro) {
		c.inDoubtHandler = fn
	}
}

// WithRiskPolicy attaches a reactive exit policy to the strategy named name. On
// every tick a monitor evaluates that strategy's attributed position against the
// policy and, when a stop-loss, trailing-stop, or take-profit trigger fires,
// submits a market exit order on the strategy's behalf.
//
// name must match a strategy's Name(); Start rejects a policy for an unknown
// strategy. Calling it again for the same name replaces the previous policy.
func WithRiskPolicy(name string, p risk.Policy) Option {
	return func(c *Cerebro) {
		if c.policies == nil {
			c.policies = map[string]risk.Policy{}
		}
		c.policies[name] = p
	}
}
