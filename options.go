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
	"time"

	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
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

func WithLogLevel(lvl log.Level) Option {
	return func(c *Cerebro) {
		c.logLevel = lvl
	}
}

func WithStrategy(st ...strategy.Strategy) Option {
	return func(c *Cerebro) {
		c.strategies = st
	}
}

func WithStrategyTimeout(du time.Duration) Option {
	return func(c *Cerebro) {
		c.timeout = du
	}
}

// WithRisk installs a pre-trade risk gate built from the given rules. Without it
// there is no gate (existing behavior). The kill switch is reachable at runtime
// via Cerebro.Kill / Cerebro.Resume.
func WithRisk(rules ...risk.Rule) Option {
	return func(c *Cerebro) {
		c.risk = risk.New(rules...)
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
