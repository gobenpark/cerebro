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
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gobenpark/trader/analysis"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/chart"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/internal/pkg"
	"github.com/gobenpark/trader/item"
	"github.com/gobenpark/trader/log"
	"github.com/gobenpark/trader/observer"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

type Filter func(item item.Item) string

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	targetCodes []string
	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`
	//isLive use cerebro live trading
	isLive bool
	// preload bool value, decide use candle history
	preload bool
	// broker buy, sell and manage order
	broker broker.Broker `validate:"required"`

	filters []Filter

	strategies []strategy.Strategy

	store store.Store

	//compress compress info map for codes
	//compress map[string][]container.CompressInfo

	// containers list of all container
	//containers map[container.Info]container.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.Engine

	//log in cerebro global logger
	Logger log.Logger `validate:"required"`

	analyzer analysis.Analyzer

	//event channel of all event
	order chan order.Order

	// eventEngine engine of management all event
	eventEngine *event.Engine

	// dataCh all data container channel
	dataCh chan container.Container

	o observer.Observer

	chart *chart.TraderChart

	tickCh map[string]chan container.Tick
}

//NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, os.Interrupt, syscall.SIGHUP)
	c := &Cerebro{
		Ctx:         ctx,
		Cancel:      cancel,
		order:       make(chan order.Order, 1),
		dataCh:      make(chan container.Container, 1),
		eventEngine: event.NewEventEngine(),
		chart:       chart.NewTraderChart(),
		tickCh:      make(map[string]chan container.Tick),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.Logger == nil {
		c.Logger = log.NewZapLogger()
	}

	if c.broker == nil {
		c.broker = broker.NewBroker(c.Logger, c.store, c.eventEngine)
	}

	if c.strategyEngine == nil {
		c.strategyEngine = strategy.NewEngine(c.Logger, c.broker)
	}

	return c
}

// SetFilter for filter market codes
func (c *Cerebro) SetFilter(f Filter) {
	c.filters = append(c.filters, f)
}

// SetStrategy set user private strategy
func (c *Cerebro) SetStrategy(s strategy.Strategy) {
	c.strategies = append(c.strategies, s)
}

//Start run cerebro
func (c *Cerebro) Start() error {
	var mu sync.Mutex
	c.Logger.Info("Cerebro starting ...")
	for _, i := range c.targetCodes {
		c.tickCh[i] = make(chan container.Tick, 1)
	}
	go pkg.Retry(3, func() error {

		tk, err := c.store.Tick(c.Ctx, c.targetCodes...)
		if err != nil {
			c.Logger.Error(err)
			return err
		}

		go func() {
			for i := range tk {
				mu.Lock()
				c.tickCh[i.Code] <- i
				mu.Unlock()
			}
		}()
		return nil
	})

	c.strategyEngine.AddStrategy(c.strategies...)

	mu.Lock()
	for code, ch := range c.tickCh {
		go c.strategyEngine.Spawn(c.Ctx, code, ch)
	}
	mu.Unlock()

	//event engine settings
	{
		go c.eventEngine.Start(c.Ctx)
		c.eventEngine.Register <- c.strategyEngine
		c.eventEngine.Register <- c.analyzer
	}

	<-c.Ctx.Done()

	return nil
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
