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
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	log2 "github.com/gobenpark/cerebro/log/v1"
	"github.com/gobenpark/cerebro/observer"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/signals"
	"github.com/gobenpark/cerebro/store"
	"github.com/gobenpark/cerebro/strategy"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	logLevel log.Level `json:"log_level,omitempty"`
	// preload bool value, decide use candle history
	preload bool `json:"preload,omitempty"`
	// broker buy, sell and manage order
	broker *broker.Broker `validate:"required" json:"broker,omitempty"`

	inmemory bool `json:"inmemory,omitempty"`

	target []item.Item `json:"target,omitempty"`

	store          store.Store   `json:"store,omitempty"`
	strategyEngine engine.Engine `json:"strategy_engine,omitempty"`

	log log.Logger `validate:"required" json:"log,omitempty"`

	analyzer analysis.Analyzer `json:"analyzer,omitempty"`

	o observer.Observer `json:"o,omitempty"`

	signalEngine engine.Engine
	// broker buy, sell and manage order
	order chan order.Order `json:"order,omitempty"`

	// eventEngine engine of management all event
	eventEngine *event.Engine `json:"event_engine,omitempty"`

	tickCh map[string]chan indicator.Tick `json:"tick_ch,omitempty"`

	cache *badger.DB `json:"cache,omitempty"`

	strategies []strategy.Strategy `json:"strategies,omitempty"`

	cash int64 `json:"cash,omitempty"`

	timeout time.Duration `json:"timeout,omitempty"`

	automaticTarget bool
}

// NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	c := &Cerebro{
		order:       make(chan order.Order, 1),
		eventEngine: event.NewEventEngine(),
		//chart:        chart.NewTraderChart(),
		tickCh: make(map[string]chan indicator.Tick),
	}

	for _, opt := range opts {
		opt(c)
	}

	db, err := badger.Open(func() badger.Options {
		if c.inmemory {
			return badger.DefaultOptions("").WithInMemory(true)
		} else {
			return badger.DefaultOptions("cerebro")
		}
	}())
	if err != nil {
		panic(err)
	}
	c.cache = db

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

	c.broker = broker.NewBroker(c.eventEngine, c.store, c.log)

	c.signalEngine = signals.NewEngine()

	if c.strategyEngine == nil {
		c.strategyEngine = strategy.NewEngine(c.log, c.eventEngine, c.broker, c.strategies, c.store, c.cache, c.timeout)
	}

	return c
}

// Start run cerebro
func (c *Cerebro) Start(ctx context.Context) error {
	c.log.Debug("Cerebro starting ...")

	if len(c.target) == 0 {
		return fmt.Errorf("error need target setting")
	}

	if c.strategies == nil {
		return fmt.Errorf("error empty strategies")
	}

	//tks := lo.FanOut(2, 1, tk)
	if err := c.strategyEngine.Spawn(ctx, c.target); err != nil {
		c.log.Error("spawn error", "err", err)
		return err
	}

	go func() {
	Done:
		for {
			select {
			case e := <-c.store.Events():
				c.eventEngine.BroadCast(e)
			case <-ctx.Done():
				break Done
			}
		}
	}()

	// event engine settings
	go c.eventEngine.Start(ctx)
	c.eventEngine.Register <- c.strategyEngine
	//c.eventEngine.Register <- c.analyzer

	return nil
}
