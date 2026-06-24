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
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

type Option func(*Cerebro)

func WithMarket(s market.Market) Option {
	return func(c *Cerebro) {
		c.market = s
	}
}

func WithTargetItem(codes ...*item.Item) Option {
	return func(c *Cerebro) {
		c.target = codes
	}
}

// WithScreener supplies the trading watchlist dynamically: at Start, Cerebro calls
// Screen and merges the returned items into the target set (deduped), so
// WithStrategyForEach spawns a strategy per selected item. It composes with
// WithTargetItem — the union is traded. This is the seam that connects screening
// ("what to trade") to strategy execution ("when to trade").
func WithScreener(s Screener) Option {
	return func(c *Cerebro) {
		c.screener = s
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

// WithStrategy registers one strategy instance. The codes select the universe it
// trades together — give two or more for a pairs/portfolio strategy that decides
// over them in one Run; give none to trade the whole target set. Every code must
// be a registered target item (WithTargetItem). Call it again to add more
// strategies. Orders are attributed to the strategy's Name(), which must be
// unique across all registrations.
func WithStrategy(s strategy.Strategy, codes ...string) Option {
	return func(c *Cerebro) {
		c.stratRegs = append(c.stratRegs, stratReg{s: s, codes: codes})
	}
}

// WithStrategyForEach registers a strategy per target item: the factory is called
// once for each WithTargetItem, producing a fresh instance with a single-item
// universe. Use it to run the same logic independently across a watchlist with
// isolated per-instance state. The factory must give each instance a unique
// Name() (e.g. derived from the item's Code) so orders attribute and route
// correctly.
func WithStrategyForEach(factory func(*item.Item) strategy.Strategy) Option {
	return func(c *Cerebro) {
		c.forEachRegs = append(c.forEachRegs, forEachReg{factory: factory})
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
