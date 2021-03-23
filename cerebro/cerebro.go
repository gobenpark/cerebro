package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
)

type Cerebro struct {
	mu sync.RWMutex
	//isLive flog of live trading
	isLive bool

	//Broker buy, sell and manage order
	broker domain.Broker `json:"broker" validate:"required"`

	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	//strategies
	strategies []domain.Strategy `json:"strategis" validate:"gte=1,dive,required"`

	//stores external api, etc
	//store can inject into cerebro what external api or oher tick,candle buy,sell history data
	stores []domain.Store

	compress map[string][]CompressInfo

	containers []domain.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.StrategyEngine

	//log in cerebro global logger
	log zerolog.Logger `json:"log" validate:"required"`

	//event channel of all event
	order chan order.Order

	eventEngine *event.EventEngine

	preload bool

	data chan domain.Container

	storengine *store.StoreEngine
}

//NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...CerebroOption) *Cerebro {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:            ctx,
		Cancel:         cancel,
		log:            logger,
		compress:       make(map[string][]CompressInfo),
		strategyEngine: &strategy.StrategyEngine{},
		order:          make(chan order.Order, 1),
		data:           make(chan domain.Container, 1),
		eventEngine:    event.NewEventEngine(),
		storengine:     new(store.StoreEngine),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Cerebro) getContainer(code string, level time.Duration) domain.Container {
	for k, v := range c.containers {
		if v.Code() == code && v.Level() == level {
			return c.containers[k]
		}
	}
	return nil
}

//load initializing data from injected store interface
func (c *Cerebro) load() error {
	if c.preload {
		var wg sync.WaitGroup
		wg.Add(len(c.stores))
		for _, i := range c.stores {
			go func(store domain.Store) {
				defer wg.Done()

				for _, comp := range c.compress[store.Uid()] {
					candles, err := store.LoadHistory(c.Ctx, comp.level)
					if err != nil {
						c.log.Err(err).Send()
						return
					}
					con := c.getContainer(store.Code(), comp.level)
					for _, candle := range candles {
						con.Add(candle)
					}
				}
			}(i)
		}
		wg.Wait()
	}
	c.log.Info().Msg("start load live data ")
	if c.isLive {
		if len(c.stores) == 0 {
			return ErrStoreNotExists
		}
		for _, i := range c.stores {
			//store per generate resample with containers
			go func(store domain.Store) {
				tick, err := store.LoadTick(c.Ctx)
				if err != nil {
					c.log.Err(err).Send()
				}

				for _, v := range c.compress[store.Uid()] {
					if container := c.getContainer(store.Code(), v.level); container != nil {
						go func(level time.Duration, isedge bool) {
							for j := range Compression(tick, level, isedge) {
								container.Add(j)
								c.data <- container
							}
						}(v.level, v.LeftEdge)
					}
				}
			}(i)
		}
	}
	return nil
}

func (c *Cerebro) registEvent() {
	c.eventEngine.Register <- c.strategyEngine
	c.eventEngine.Register <- c.storengine
}

//Start run cerebro
// first check cerebro validation
// second load from store data
// third other engine setup
func (c *Cerebro) Start() error {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGKILL)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.log.Err(err).Send()
		return err
	}

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
