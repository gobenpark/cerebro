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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/container"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/internal/pkg"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	log2 "github.com/gobenpark/cerebro/log/v1"
	"github.com/gobenpark/cerebro/observer"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/store"
	"github.com/gobenpark/cerebro/strategy"
	"go.uber.org/zap"
)

type Filter func(item item.Item) string

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	logLevel log.Level
	//isLive use cerebro live trading
	isLive bool
	// preload bool value, decide use candle history
	preload bool
	// broker buy, sell and manage order
	broker *broker.Broker `validate:"required"`

	filters []Filter

	strategies []strategy.Strategy

	target []string

	controlPlane *container.ControlPlane

	store store.Store
	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.Engine

	//log in cerebro global logger
	log log.Logger `validate:"required"`

	analyzer analysis.Analyzer

	//event channel of all event
	order chan order.Order

	// eventEngine engine of management all event
	eventEngine *event.Engine

	// dataCh all data container channel
	dataCh chan container.Container

	o observer.Observer

	//chart *chart.TraderChart

	tickCh map[string]chan container.Tick

	commision float64

	cash int64

	mu sync.RWMutex

	timeout time.Duration
}

// NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	c := &Cerebro{
		order:       make(chan order.Order, 1),
		dataCh:      make(chan container.Container, 1),
		eventEngine: event.NewEventEngine(),
		//chart:        chart.NewTraderChart(),
		tickCh: make(map[string]chan container.Tick),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.log == nil {
		if c.logLevel == 0 {
			c.logLevel = log.InfoLevel
		}

		log, err := log2.NewLogger(c.logLevel)
		if err != nil {
			panic(err)
		}
		c.log = log
	}
	c.controlPlane = container.NewControlPlane(c.log)

	c.broker = broker.NewBroker(c.eventEngine, c.store, c.commision, c.cash, c.log)

	if c.strategyEngine == nil {
		c.strategyEngine = strategy.NewEngine(c.log, c.broker, c.preload, c.timeout)
	}

	return c
}

// SetFilter for filter market codes
func (c *Cerebro) SetFilter(f Filter) {
	c.filters = append(c.filters, f)
}

// Start run cerebro
func (c *Cerebro) Start(ctx context.Context) error {
	c.log.Debug("Cerebro starting ...")

	if len(c.target) == 0 {
		return fmt.Errorf("error target zero value")
	}

	if c.strategies == nil {
		return fmt.Errorf("error empty strategies")
	}

	for _, i := range c.target {
		c.tickCh[i] = make(chan container.Tick, 1)
	}

	if c.preload {
	}

	c.strategyEngine.AddStrategy(c.strategies...)

	// tick data receive from store
	go pkg.Retry(ctx, 3, func() error {
		tk, err := c.store.Tick(ctx, c.target...)
		if err != nil {
			c.log.Error("store tick error", zap.Error(err))
			return err
		}

		ch := c.controlPlane.Add(pkg.OrDone(ctx, tk))

		if err := c.strategyEngine.Spawn(ctx, ch); err != nil {
			c.log.Error("spawn error", "err", err)
			return err
		}

		return nil
	})

	//event engine settings
	{
		go c.eventEngine.Start(ctx)
		c.eventEngine.Register <- c.strategyEngine
		c.eventEngine.Register <- c.analyzer
	}

	return nil
}
