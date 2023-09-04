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
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/analysis"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/container"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	log2 "github.com/gobenpark/cerebro/log/v1"
	"github.com/gobenpark/cerebro/observer"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/store"
	"github.com/gobenpark/cerebro/strategy"
	"github.com/reactivex/rxgo/v2"
	"go.uber.org/zap"
)

type Filter func(item item.Item) string

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	logLevel log.Level `json:"log_level,omitempty"`

	isLive bool `json:"is_live,omitempty"`
	// preload bool value, decide use candle history
	preload bool `json:"preload,omitempty"`
	// broker buy, sell and manage order
	broker *broker.Broker `validate:"required" json:"broker,omitempty"`

	filters []Filter `json:"filters,omitempty"`

	strategies []strategy.Strategy `json:"strategies,omitempty"`

	target []item.Item `json:"target,omitempty"`

	controlPlane *container.ControlPlane `json:"control_plane,omitempty"`

	store          store.Store      `json:"store,omitempty"`
	strategyEngine *strategy.Engine `json:"strategy_engine,omitempty"`

	log log.Logger `validate:"required" json:"log,omitempty"`

	analyzer analysis.Analyzer `json:"analyzer,omitempty"`

	order chan order.Order `json:"order,omitempty"`

	// eventEngine engine of management all event
	eventEngine *event.Engine `json:"event_engine,omitempty"`

	// dataCh all data container channel
	dataCh chan container.Container `json:"data_ch,omitempty"`

	o observer.Observer `json:"o,omitempty"`

	tickCh map[string]chan container.Tick `json:"tick_ch,omitempty"`

	commision float64 `json:"commision,omitempty"`

	cash int64 `json:"cash,omitempty"`

	mu sync.RWMutex `json:"mu"`

	timeout time.Duration `json:"timeout,omitempty"`

	cache *badger.DB `json:"cache,omitempty"`

	automaticTarget bool
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

		logger, err := log2.NewLogger(c.logLevel)
		if err != nil {
			panic(err)
		}
		c.log = logger
	}
	c.controlPlane = container.NewControlPlane(c.log)

	c.broker = broker.NewBroker(c.eventEngine, c.store, c.commision, c.cash, c.log)

	if c.strategyEngine == nil {
		c.strategyEngine = strategy.NewEngine(c.log, c.broker, c.preload, c.cache, c.timeout)
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

	if len(c.target) == 0 || !c.automaticTarget {
		return fmt.Errorf("error need target setting")
	}

	if c.strategies == nil {
		return fmt.Errorf("error empty strategies")
	}

	if c.automaticTarget {
		//c.target = lo.Map[item.Item, string](c.store.MarketItems(ctx), func(item item.Item, index int) string {
		//	return item.Code
		//})
	}

	//for _, i := range c.target {
	//	c.tickCh[i] = make(chan container.Tick, 1)
	//}

	if c.preload {
		//TODO: preload
	}

	c.strategyEngine.AddStrategy(c.strategies...)

	tk, err := c.store.Tick(ctx, c.target...)
	if err != nil {
		c.log.Error("store tick error", zap.Error(err))
		return err
	}

	//ch := c.controlPlane.Add(pkg.OrDone(ctx, tk))

	rxch := make(chan rxgo.Item)
	go func() {
		for i := range tk {
			rxch <- rxgo.Of(i)
		}
	}()

	observer := rxgo.FromEventSource(rxch, rxgo.WithContext(ctx))
	if err := c.strategyEngine.Spawn(ctx, c.target, observer); err != nil {
		c.log.Error("spawn error", "err", err)
		return err
	}

	// event engine settings
	go c.eventEngine.Start(ctx)
	c.eventEngine.Register <- c.strategyEngine
	c.eventEngine.Register <- c.analyzer

	return nil
}
