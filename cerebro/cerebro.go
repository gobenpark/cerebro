package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/rs/zerolog"
)

type CerebroOption func(*Cerebro)

type Cerebro struct {
	//isLive flog of live trading
	isLive bool

	//Broker buy, sell and manage order
	Broker domain.Broker `json:"broker" validate:"required"`

	Ctx context.Context `json:"ctx" validate:"required"`

	Cancel context.CancelFunc `json:"cancel" validate:"required"`
	//Strategies
	Strategies []domain.Strategy `json:"strategis" validate:"gte=1,dive,required"`

	Store domain.Store

	//Feeds
	Feeds []domain.Feed

	Log zerolog.Logger `json:"log" validate:"required"`

	//event channel of all event
	event chan event.Event

	order chan order.Order
}

func NewCerebro(opts ...CerebroOption) *Cerebro {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:    ctx,
		Cancel: cancel,
		Log:    logger,
		event:  make(chan event.Event, 1),
		order:  make(chan order.Order),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Cerebro) startFeeds() {
	for _, f := range c.Feeds {
		f.AddStore(c.Store)
		f.Start(true, true, c.Strategies)
	}
}

func (c *Cerebro) Start() error {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGKILL)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.Log.Err(err).Send()
		return err
	}

	ech := []chan event.Event{}
	go func() {
	Done:
		for {
			select {
			case <-c.Ctx.Done():
				c.Log.Info().Msg("done")
				break Done
			case e, ok := <-c.event:
				fmt.Println(e)
				if ok {
					for _, i := range ech {
						i <- e
					}
				}
			}
		}
	}()

	c.Broker.SetEventCh(c.event)
	c.Log.Info().Msg("Cerebro start...")

	for _, i := range c.Strategies {
		ch := make(chan event.Event)
		go i.Start(c.Ctx, ch)
		ech = append(ech, ch)
	}

	c.startFeeds()
	<-ch
	return nil
}

func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
