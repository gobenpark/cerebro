package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
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

	compress map[string]CompressInfo

	containers map[string]domain.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.StrategyEngine

	//log in cerebro global logger
	log zerolog.Logger `json:"log" validate:"required"`

	//event channel of all event
	event chan event.Event

	order chan order.Order

	eventEngine *event.EventEngine

	preload bool

	data chan domain.Container

	events []chan event.Event
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
		containers:     make(map[string]domain.Container),
		compress:       make(map[string]CompressInfo),
		strategyEngine: &strategy.StrategyEngine{},
		event:          make(chan event.Event, 1),
		order:          make(chan order.Order, 1),
		data:           make(chan domain.Container, 1),
		eventEngine:    event.NewEventEngine(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

//load initializing data from injected store interface
func (c *Cerebro) load() error {
	if c.preload {
		var wg sync.WaitGroup
		wg.Add(len(c.stores))
		for _, i := range c.stores {
			go func(store domain.Store) {
				defer wg.Done()

				var level time.Duration
				c.mu.Lock()
				if com, ok := c.compress[store.Uid()]; ok {
					level = com.level
				}

				c.mu.Unlock()

				candle, err := store.LoadHistory(c.Ctx, level)
				if err != nil {
					c.log.Err(err).Send()
					return
				}
				c.mu.Lock()
				if _, ok := c.containers[store.Uid()]; !ok {
					c.containers[store.Uid()] = datacontainer.NewDataContainer(store.Code())
				}
				for _, j := range candle {
					c.containers[store.Uid()].Add(j)
				}

				for _, i := range c.containers[store.Uid()].Values() {
					fmt.Println(i.Date)
				}
				c.mu.Unlock()
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
			go func(store domain.Store) {
				tick, err := store.LoadTick(c.Ctx)
				if err != nil {
					c.log.Err(err).Send()
				}
				c.mu.Lock()
				if _, ok := c.containers[store.Uid()]; !ok {
					c.containers[store.Uid()] = datacontainer.NewDataContainer(store.Code())
				}
				c.mu.Unlock()
				com := c.compress[store.Uid()]
				for j := range Compression(tick, com.level, com.LeftEdge) {
					c.containers[store.Uid()].Add(j)
					c.data <- c.containers[store.Uid()]
				}
			}(i)
		}

		<-c.Ctx.Done()
	}
	return nil
}
func (c *Cerebro) registEvent() {
	c.eventEngine.Register <- c.strategyEngine
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
