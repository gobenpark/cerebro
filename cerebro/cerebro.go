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
	"os/signal"
	"syscall"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/chart"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
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

	targetCodes []string
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

func (c *Cerebro) liveStrategyStart() {

	for _, i := range c.store.GetMarketItems() {
		tk, err := c.store.Tick(c.Ctx, i.Code)
		if err != nil {
			c.Logger.Error(err)
			continue
		}

		go func() {
			for _, st := range c.strategies {
				switch st.CandleType() {
				case strategy.Min3:
					go func() {
						con := container.NewDataContainer(container.Info{
							Code:             i.Code,
							CompressionLevel: 3 * time.Minute,
						})
						for candle := range Compression(tk, 3*time.Minute, true) {
							con.Add(candle)
							st.Next(c.broker, con)
						}
					}()

				case strategy.Day:
					go func() {
						con := container.NewDataContainer(container.Info{
							Code:             i.Code,
							CompressionLevel: 24 * time.Hour,
						})
						for candle := range Compression(tk, 24*time.Hour, true) {
							con.Add(candle)
							st.Next(c.broker, con)
						}
					}()
				}
			}
		}()
	}
}

func (c *Cerebro) startPastStrategy() {
	for _, i := range c.store.GetMarketItems() {
		ctx, cancel := context.WithTimeout(c.Ctx, 5*time.Second)

		candle, err := c.store.Candles(ctx, i.Code, store.DAY, 1)
		if err != nil {
			c.Logger.Error(err)
			continue
		}

		info := container.Info{
			Code:             i.Code,
			CompressionLevel: 24 * time.Hour,
		}
		if _, ok := c.containers[info]; !ok {
			c.containers[info] = container.NewDataContainer(info, candle...)
		}

		for _, st := range c.strategies {
			st.Next(c.broker, c.containers[info])
		}

		cancel()
	}
}

//Start run cerebro
func (c *Cerebro) Start() error {
	c.Logger.Info("Cerebro start ...")

	c.strategyEngine.Start(c.Ctx, c.dataCh)

	for _, i := range c.filters {
		for _, j := range c.store.GetMarketItems() {
			c.targetCodes = append(c.targetCodes, i(j))
		}
	}

	c.Logger.Info("loading...")

	if c.isLive {
		c.liveStrategyStart()
		select {
		case <-c.Ctx.Done():
			break
		}
		return nil
	} else {
		c.startPastStrategy()
		return nil
	}
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
