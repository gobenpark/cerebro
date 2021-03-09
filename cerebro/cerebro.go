package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/rs/zerolog"
)

type Cerebroker interface {
	// start cerebro trading
	Start() error

	//stop cerebro and other
	Stop() error

	//add strategy into cerebro
	AddStrategy(domain.Strategy)

	//AddData add data feed
	AddData(feed domain.Feed)
}

type cerebro struct {
	//isLive flog of live trading
	isLive bool

	Broker domain.Broker `json:"broker" validate:"required"`

	Ctx context.Context `json:"ctx" validate:"required"`

	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	Strategies []domain.Strategy `json:"strategis" validate:"gte=1,dive,required"`

	Feeds []domain.Feed

	Log zerolog.Logger `json:"log" validate:"required"`

	event chan event.Event

	order chan order.Order
}

func NewCerebro(broker domain.Broker) Cerebroker {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())
	return &cerebro{
		Broker: broker,
		Ctx:    ctx,
		Cancel: cancel,
		Log:    logger,
		event:  make(chan event.Event, 1),
		order:  make(chan order.Order),
	}
}

func (c *cerebro) AddData(feed domain.Feed) {
	c.Feeds = append(c.Feeds, feed)
}

func (c *cerebro) AddStrategy(st domain.Strategy) {
	c.Strategies = append(c.Strategies, st)
}

func (c *cerebro) startFeeds() {
	for _, f := range c.Feeds {
		f.Start(true, true, c.Strategies)
	}
}

func (c *cerebro) Start() error {
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

	return nil
}

func (c *cerebro) Stop() error {
	c.Cancel()
	return nil
}
