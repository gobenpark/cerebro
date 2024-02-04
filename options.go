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

	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/observer"
	"github.com/gobenpark/cerebro/strategy"
)

type Option func(*Cerebro)

func WithObserver(o observer.Observer) Option {
	return func(c *Cerebro) {
		c.o = o
	}
}

func WithMarket(s market.Market) Option {
	return func(c *Cerebro) {
		c.market = s
	}
}

func WithTargetItem(codes ...item.Item) Option {
	return func(c *Cerebro) {
		c.target = codes
	}
}

func WithPreload(b bool) Option {
	return func(c *Cerebro) {
		c.preload = b
	}
}

func WithAnalyzer(analyzer analysis.Analyzer) Option {
	return func(c *Cerebro) {
		c.analyzer = analyzer
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

func WithStartTime(ti string) Option {
	return func(c *Cerebro) {
		c.startTime = ti
	}
}
