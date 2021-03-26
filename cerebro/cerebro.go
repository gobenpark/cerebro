package cerebro

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	error2 "github.com/gobenpark/trader/error"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/internal/pkg/retry"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	//isLive flog of live trading
	isLive bool

	//Broker buy, sell and manage order
	broker broker.Broker `validate:"required"`

	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	//strategies
	strategies []strategy.Strategy `validate:"gte=1,dive,required"`

	//compress compress info map for codes
	compress map[string][]CompressInfo

	containers []container.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.Engine

	//log in cerebro global logger
	log zerolog.Logger `validate:"required"`

	//event channel of all event
	order chan order.Order

	eventEngine *event.Engine

	preload bool

	data chan container.Container

	storengine *store.Engine
}

//NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:            ctx,
		Cancel:         cancel,
		log:            logger,
		compress:       make(map[string][]CompressInfo),
		strategyEngine: &strategy.Engine{},
		order:          make(chan order.Order, 1),
		data:           make(chan container.Container, 1),
		eventEngine:    event.NewEventEngine(),
		storengine:     store.NewEngine(),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.storengine.EventEngine = c.eventEngine

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

//load initializing data from injected store interface
func (c *Cerebro) load() error {
	//gocyclo:ignore
	if c.preload {
		for k, v := range c.storengine.Mapper {
			for _, code := range v {
				for _, comp := range c.compress[code] {
					if err := retry.Retry(10, func() error {
						candles, err := c.storengine.Stores[k].LoadHistory(c.Ctx, code, comp.level)
						if err != nil {
							c.log.Err(err).Send()
							return err
						}
						con := c.getContainer(code, comp.level)
						for _, candle := range candles {
							con.Add(candle)
						}
						return nil
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	c.log.Info().Msg("start load live data ")
	if c.isLive {
		if len(c.storengine.Stores) == 0 {
			return error2.ErrStoreNotExists
		}
		//All Store
		for k, v := range c.storengine.Mapper {
			// all codes
			for _, i := range v {
				var tick <-chan container.Tick
				if err := retry.Retry(10, func() error {
					var err error
					tick, err = c.storengine.Stores[k].LoadTick(c.Ctx, i)
					if err != nil {
						return err
					}
					return nil
				}); err != nil {
					return err
				}

				for _, com := range c.compress[i] {
					if con := c.getContainer(i, com.level); con != nil {
						go func(t <-chan container.Tick, con container.Container, level time.Duration, isedge bool) {
							for j := range Compression(t, level, isedge) {
								con.Add(j)
								c.data <- con
							}
						}(tick, con, com.level, com.LeftEdge)
					}
				}
			}
		}
	}
	return nil
}

func (c *Cerebro) registEvent() {
	c.eventEngine.Register <- c.strategyEngine
	c.eventEngine.Register <- c.storengine
}

func (c *Cerebro) createContainer() {
	for _, v := range c.storengine.Mapper {
		for _, i := range v {
			for _, j := range c.compress[i] {
				c.containers = append(c.containers, container.NewDataContainer(container.Info{
					Code:             i,
					CompressionLevel: j.level,
				}))
			}
		}
	}
}

//Start run cerebro
// first check cerebro validation
// second load from store data
// third other engine setup
func (c *Cerebro) Start() error {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.log.Err(err).Send()
		return err
	}

	c.createContainer()

	c.eventEngine.Start(c.Ctx)
	c.registEvent()
	c.log.Info().Msg("Cerebro start...")

	c.strategyEngine.Broker = c.broker
	c.strategyEngine.Start(c.Ctx, c.data, c.strategies)

	c.log.Info().Msg("startload")
	if err := c.load(); err != nil {
		return err
	}
	c.log.Info().Msg("end load")

	select {
	case <-c.Ctx.Done():
		break
	case <-ch:
		break
	}
	return nil
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
