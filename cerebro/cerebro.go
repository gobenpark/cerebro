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
	"syscall"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/chart"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/observer"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

type Filter func() string

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	// preload bool value, decide use candle history
	preload bool

	// broker buy, sell and manage order
	broker broker.Broker `validate:"required"`

	filters []Filter

	strategies []strategy.Strategy

	store store.Store

	//compress compress info map for codes
	compress map[string][]CompressInfo

	// containers list of all container
	containers []container.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.Engine

	//log in cerebro global logger
	Logger Logger `validate:"required"`

	//event channel of all event
	order chan order.Order

	// eventEngine engine of management all event
	eventEngine *event.Engine

	// dataCh all data container channel
	dataCh chan container.Container

	o observer.Observer

	chart *chart.TraderChart
}

//NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:            ctx,
		Cancel:         cancel,
		compress:       make(map[string][]CompressInfo),
		strategyEngine: &strategy.Engine{},
		order:          make(chan order.Order, 1),
		dataCh:         make(chan container.Container, 1),
		eventEngine:    event.NewEventEngine(),
		chart:          chart.NewTraderChart(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.broker == nil {
		c.broker = broker.NewBroker(c.store)
	}

	if c.Logger == nil {
		c.Logger = GetLogger()
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

func (c *Cerebro) load() {
	items := c.store.GetMarketItems()
	_ = items
}

//Start run cerebro
func (c *Cerebro) Start() error {
	done := make(chan os.Signal)
	signal.Notify(done, syscall.SIGTERM)

	//c.createContainer()
	//c.chart.Start()

	//c.eventEngine.Start(c.Ctx)
	//c.registerEvent()
	c.Logger.Info("Cerebro start ...")
	//c.broker.Store = c.store
	//c.strategyEngine.Broker = c.broker
	c.strategyEngine.Start(c.Ctx, c.dataCh)

	//c.broker.SetEventBroadCaster(c.eventEngine)

	c.Logger.Info("loading...")

	select {
	case <-c.Ctx.Done():
		break
	case <-done:
		break
	}
	return nil
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
