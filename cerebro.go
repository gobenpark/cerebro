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
	"fmt"
	"slices"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	log2 "github.com/gobenpark/cerebro/log/v1"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/observer"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/strategy"
	"github.com/samber/lo"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	cancel   context.CancelFunc
	logLevel log.Level
	// preload bool value, decide use candle history
	preload bool
	// broker buy, sell and manage order
	broker *broker.Broker

	target []item.Item

	market         market.Market
	strategyEngine engine.Engine

	log log.Logger

	analyzerEngine *analysis.Engine

	analyzer analysis.Analyzer

	o observer.Observer

	signalEngine engine.Engine
	// broker buy, sell and manage order
	order chan order.Order
	// eventEngine engine of management all event
	eventEngine *event.Engine

	cache *badger.DB

	strategies []strategy.Strategy

	timeout time.Duration

	startTime string
}

// NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	c := &Cerebro{
		order:       make(chan order.Order, 1),
		eventEngine: event.NewEventEngine(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.log == nil {
		if c.logLevel == 0 {
			c.logLevel = log.InfoLevel
		}

		logger, err := log2.NewLogger(c.logLevel)
		if err != nil {
			panic(err)
		}
		c.log = logger
	}

	c.broker = broker.NewDefaultBroker(c.eventEngine, c.market, c.log)
	c.strategyEngine = strategy.NewEngine(c.log, c.eventEngine, c.broker, c.strategies, c.market, c.cache, c.timeout)
	c.analyzerEngine = analysis.NewEngine(c.log)
	c.analyzerEngine.Analyzer = c.analyzer

	return c
}

// Start run cerebro
func (c *Cerebro) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.log.Info("Cerebro starting ...")

	if len(c.target) == 0 {
		return fmt.Errorf("error need target setting")
	}

	if c.strategies == nil {
		return fmt.Errorf("error empty strategies")
	}

	positions := c.market.AccountPositions()
	filterd := []item.Item{}
	for i := range c.strategies {
		for j := range c.target {
			prd := strategy.NewCandleProvider(c.market, c.target[j])
			if c.strategies[i].Pass(c.target[j], prd) || slices.ContainsFunc(positions, func(position position.Position) bool { return position.Item.Code == c.target[j].Code }) {
				filterd = append(filterd, c.target[j])
			}
		}
	}

	tk, err := c.market.Tick(ctx, filterd...)
	if err != nil {
		c.log.Error("store tick error", "error", err)
		return err
	}

	ticks := lo.FanOut(2, 1, tk)

	go func() {
		if err := c.strategyEngine.Spawn(ctx, filterd, ticks[0]); err != nil {
			c.log.Error("spawn error", "err", err)
			return
		}
	}()

	go func() {
		if err := c.analyzerEngine.Spawn(ctx, filterd, ticks[1]); err != nil {
			c.log.Error("analyzer spawn error", "err", err)
			return
		}
	}()

	go func() {
		ch := c.market.Events(ctx)
	Done:
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					c.log.Info("event channel closed")
					break Done
				}
				c.eventEngine.BroadCast(e)
			case <-ctx.Done():
				c.log.Info("context done")
				break Done
			}
		}
	}()
	// event engine settings
	go c.eventEngine.Start(ctx)
	c.eventEngine.Register <- c.strategyEngine
	c.eventEngine.Register <- c.broker

	return nil
}

func (c *Cerebro) Shutdown() {
	c.log.Info("shutdown")
	c.cancel()
}
