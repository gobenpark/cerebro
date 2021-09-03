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
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/chart"
	"github.com/gobenpark/trader/container"
	error2 "github.com/gobenpark/trader/error"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/internal/pkg"
	"github.com/gobenpark/trader/observer"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	//Broker buy, sell and manage order
	broker *broker.Broker `validate:"required"`

	store store.Store

	codes []string

	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	//strategies
	strategies []strategy.Strategy `validate:"gte=1,dive,required"`

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

	// preload bool value, decide use candle history
	preload bool

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
	if c.Logger == nil {
		c.Logger = GetLogger()
	}

	return c
}

func (c *Cerebro) getContainer(code string, level time.Duration) container.Container {
	for k, v := range c.containers {
		if v.Code() == code && v.Level() == level {
			return c.containers[k]
		}
	}
	return nil
}

// orderEventRoutine is stream of order state
// if rise order event then event hub send to subscriber
func (c *Cerebro) orderEventRoutine() {
	ch, err := c.store.OrderState(c.Ctx)
	if err != nil {
		panic(err)
	}

	go func() {
		for i := range ch {
			c.eventEngine.BroadCast(i)
		}
	}()
}

//load initializing data from injected store interface
func (c *Cerebro) load() error {
	//preload is load history data
	//gocyclo:ignore
	if c.preload {
		for _, code := range c.codes {
			for _, comp := range c.compress[code] {
				if err := pkg.Retry(10, func() error {
					candles, err := c.store.LoadHistory(c.Ctx, code, comp.level)
					if err != nil {
						c.Logger.Error(err)
						return err
					}
					con := c.getContainer(code, comp.level)
					for _, candle := range candles {
						con.Add(candle)
					}

					select {
					case c.chart.Input <- con:
					case <-c.Ctx.Done():
					}
					return nil
				}); err != nil {
					return err
				}
			}
		}
	}

	// getting live trading data like tick data
	c.Logger.Info("start load live data")
	if c.store == nil {
		return error2.ErrStoreNotExists
	}

	for _, i := range c.codes {
		var tick <-chan container.Tick
		if err := pkg.Retry(10, func() error {
			var err error
			tick, err = c.store.LoadTick(c.Ctx, i)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		for _, com := range c.compress[i] {
			if con := c.getContainer(i, com.level); con != nil {
				go func(t <-chan container.Tick, con container.Container, level time.Duration, isLeftEdge bool) {
					com, oth := pkg.Tee(c.Ctx, t)

					go func(ch <-chan container.Tick) {
						for o := range ch {
							if c.o != nil {
								c.o.Next(o)
							}
						}
					}(oth)

					for j := range Compression(com, level, isLeftEdge) {
						con.Add(j)
						select {
						case <-c.Ctx.Done():
							break
						default:
							c.dataCh <- con
							c.chart.Input <- con
						}
					}
				}(tick, con, com.level, com.LeftEdge)
			}
		}
	}
	return nil
}

// registerEvent is resiter event listener
func (c *Cerebro) registerEvent() {
	c.eventEngine.Register <- c.strategyEngine
	//c.eventEngine.Register <- c.broker
}

func (c *Cerebro) createContainer() {
	for _, i := range c.codes {
		c.containers = append(c.containers, container.NewDataContainer(container.Info{Code: i, CompressionLevel: time.Second * 0}))
		for _, j := range c.compress[i] {
			c.containers = append(c.containers, container.NewDataContainer(container.Info{
				Code:             i,
				CompressionLevel: j.level,
			}))
		}
	}
}

//Start run cerebro
// first check cerebro validation
// second load from store data
// third other engine setup
func (c *Cerebro) Start() error {
	done := make(chan os.Signal)
	signal.Notify(done, syscall.SIGTERM)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.Logger.Error(err)
		return err
	}

	c.createContainer()
	c.chart.Start()

	c.eventEngine.Start(c.Ctx)
	c.registerEvent()
	c.Logger.Info("Cerebro start ...")
	//c.broker.Store = c.store
	c.strategyEngine.Broker = c.broker
	c.strategyEngine.Start(c.Ctx, c.dataCh)

	//c.broker.SetEventBroadCaster(c.eventEngine)

	c.orderEventRoutine()
	c.Logger.Info("loading...")
	if err := c.load(); err != nil {
		return err
	}

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
