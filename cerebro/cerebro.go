package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/feeds"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
)

type CerebroOption func(*Cerebro)

type Cerebro struct {
	//isLive flog of live trading
	isLive bool
	//Broker buy, sell and manage order
	broker domain.Broker `json:"broker" validate:"required"`

	Ctx context.Context `json:"ctx" validate:"required"`

	Cancel context.CancelFunc `json:"cancel" validate:"required"`
	//strategies
	strategies []domain.Strategy `json:"strategis" validate:"gte=1,dive,required"`

	store domain.Store
	//Feeds
	Feeds []domain.Feed

	feeds.FeedEngine

	sengine strategy.StrategyEngine

	log zerolog.Logger `json:"log" validate:"required"`
	//event channel of all event
	event chan event.Event

	order chan order.Order

	compress time.Duration

	preload bool
}

func NewCerebro(opts ...CerebroOption) *Cerebro {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:    ctx,
		Cancel: cancel,
		log:    logger,
		event:  make(chan event.Event, 1),
		order:  make(chan order.Order, 1),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Cerebro) load() {
	if c.preload {
		cd, err := c.store.LoadHistory(c.Ctx, "")
		if err != nil {
			c.log.Err(err).Send()
			return
		}
	}

	if c.isLive {
		ch, err := c.store.LoadTick(c.Ctx, "")
		if err != nil {
			c.log.Err(err).Send()
			return
		}
		//Compression(ch,time.Minute * 3)
	}
}

func (c *Cerebro) Start() error {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGKILL)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.log.Err(err).Send()
		return err
	}
	c.log.Info().Msg("Cerebro start...")

	c.load()

	c.sengine.Start(c.Ctx)
	c.broker.SetEventCh(c.event)
	<-ch
	return nil
}

func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
