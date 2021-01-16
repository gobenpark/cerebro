package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/store/model"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
	"os"
	"time"
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
	Broker     broker.Broker       `json:"broker" validate:"required"`
	Store      []store.Storer      `json:"store" validate:"gte=1,dive,required"`
	Ctx        context.Context     `json:"ctx" validate:"required"`
	Cancel     context.CancelFunc  `json:"cancel" validate:"required"`
	Strategies []strategy.Strategy `json:"strategis" validate:"gte=1,dive,required"`
	ChartData  chan model.Chart
	Log        zerolog.Logger `json:"log" validate:"required"`
}

func NewCerebro(broker broker.Broker) Cerebroker {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	ctx, cancel := context.WithCancel(context.Background())
	return &cerebro{
		Broker:    broker,
		Store:     []store.Storer{},
		Ctx:       ctx,
		Cancel:    cancel,
		ChartData: make(chan model.Chart, 1000),
		Log:       logger,
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

	return nil
}

func (c *cerebro) AddStore(store store.Storer) {
	c.Store = append(c.Store, store)
}

func (c *cerebro) Stop() error {
	c.Cancel()
	return nil
}

func (c *cerebro) Resample() {

}
