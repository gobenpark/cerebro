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
	"os/signal"
	"syscall"
	"time"

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
	compress map[string][]CompressInfo

	// containers list of all container
	containers map[container.Info]container.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.Engine

	//log in cerebro global logger
	Logger log.Logger `validate:"required"`

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
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	c := &Cerebro{
		Ctx:            ctx,
		Cancel:         cancel,
		compress:       make(map[string][]CompressInfo),
		containers:     make(map[container.Info]container.Container),
		strategyEngine: &strategy.Engine{},
		order:          make(chan order.Order, 1),
		dataCh:         make(chan container.Container, 1),
		eventEngine:    event.NewEventEngine(),
		chart:          chart.NewTraderChart(),
		tickCh:         make(map[string]chan container.Tick),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.broker == nil {
		c.broker = broker.NewBroker(c.store)
	}

	if c.Logger == nil {
		c.Logger = log.NewZapLogger()
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
	c.Logger.Info("Cerebro starting ...")

	go pkg.Retry(3, func() error {

		for _, i := range c.targetCodes {
			c.tickCh[i] = make(chan container.Tick, 1)
		}

		tk, err := c.store.Tick(c.Ctx, c.targetCodes...)
		if err != nil {
			c.Logger.Error(err)
			return err
		}

		go func() {
			for i := range tk {
				c.tickCh[i.Code] <- i
			}
		}()

		for _, v := range c.tickCh {
			go func(ch <-chan container.Tick) {
				for i := range Compression(ch, time.Minute, true) {
					if i.Code == "KRW-SSX" {
						fmt.Println(i.Open)
						fmt.Println(i.High)
						fmt.Println(i.Low)
						fmt.Println(i.Close)
						fmt.Println(i.Volume)
						fmt.Println(i.Date)
					}
				}
			}(v)
		}

		return nil
	})

	//for _, i := range c.filters {
	//	for _, j := range c.store.GetMarketItems() {
	//		c.targetCodes = append(c.targetCodes, i(j))
	//	}
	//}
	//
	//go c.eventEngine.Start(c.Ctx)
	//c.eventEngine.Register <- c.strategyEngine
	//
	//c.Logger.Info("loading...")
	<-c.Ctx.Done()

	return nil
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
