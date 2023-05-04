/*
 *  Copyright 2021 The Trader Authors
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
	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/observer"
	"github.com/gobenpark/cerebro/store"
)

type Option func(*Cerebro)

func WithObserver(o observer.Observer) Option {
	return func(c *Cerebro) {
		c.o = o
	}
}

func WithLive() Option {
	return func(c *Cerebro) {
		c.isLive = true
	}
}

func WithStore(s store.Store) Option {
	return func(c *Cerebro) {
		c.store = s
	}
}

func WithTargetItem(codes ...string) Option {
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

func WithCommision(com float64) Option {
	return func(c *Cerebro) {
		c.commision = com
	}
}
