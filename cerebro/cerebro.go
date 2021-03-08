package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/store/model"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
)

type Cerebroker interface {
	// start cerebro trading
	Start() error

	//stop cerebro and other
	Stop() error

	// add store into cerebro
	AddStore(store.Storer)

	//add strategy into cerebro
	AddStrategy(strategy.Strategy)

	Resample()
}

type cerebro struct {
	//isLive flog of live trading
	isLive bool

	Broker     broker.Broker       `json:"broker" validate:"required"`
	Stores     []store.Storer      `json:"store" validate:"gte=1,dive,required"`
	Ctx        context.Context     `json:"ctx" validate:"required"`
	Cancel     context.CancelFunc  `json:"cancel" validate:"required"`
	Strategies []strategy.Strategy `json:"strategis" validate:"gte=1,dive,required"`
	ChartData  chan model.Chart
	Log        zerolog.Logger `json:"log" validate:"required"`
	event      chan event.Event
	order      chan order.Order
}

func NewCerebro(broker broker.Broker) Cerebroker {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	ctx, cancel := context.WithCancel(context.Background())
	return &cerebro{
		Broker:    broker,
		Stores:    []store.Storer{},
		Ctx:       ctx,
		Cancel:    cancel,
		ChartData: make(chan model.Chart, 1000),
		Log:       logger,
		event:     make(chan event.Event),
		order:     make(chan order.Order),
	}
}

func (c *cerebro) AddStrategy(st strategy.Strategy) {
	c.Strategies = append(c.Strategies, st)
}

func (c *cerebro) Start() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.Log.Err(err).Send()
		return err
	}

	c.Log.Info().Msg("Cerebro start...")
	ech := []chan event.Event{}
	for _, i := range c.Strategies {
		ch := make(chan event.Event)
		go i.Start(c.Ctx, ch)
		ech = append(ech, ch)
	}

	go func() {
	Done:
		for {
			select {
			case <-c.Ctx.Done():
				break Done
			case e, ok := <-c.event:
				if ok {
					for _, i := range ech {
						i <- e
					}
				}
			}
		}
	}()

	return nil
}

func (c *cerebro) AddStore(store store.Storer) {
	c.Stores = append(c.Stores, store)
}

func (c *cerebro) Stop() error {
	c.Cancel()
	return nil
}

func (c *cerebro) Resample() {
}
