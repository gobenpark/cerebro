package cerebro

import (
	"context"
	"fmt"
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/store"
	"github.com/BumwooPark/trader/store/model"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type Cerebroker interface {
	Start() error
	Stop()
	AddStore(store.Storer)
}

type cerebro struct {
	Broker broker.Broker      `json:"broker" validate:"required"`
	Store  []store.Storer     `json:"store" validate:"gte=1,dive,required"`
	Ctx    context.Context    `json:"ctx" validate:"required"`
	Cancel context.CancelFunc `json:"cancel" validate:"required"`
	//Strategis []strategy.Strategy `json:"strategis" validate:"gte=1,dive,required"`
	ChartData chan model.Chart
	Log       *zap.Logger `json:"log" validate:"required"`
}

func NewCerebro(broker broker.Broker) Cerebroker {
	ctx, cancel := context.WithCancel(context.Background())
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	return &cerebro{
		Broker:    broker,
		Store:     []store.Storer{},
		Ctx:       ctx,
		Cancel:    cancel,
		ChartData: make(chan model.Chart, 1000),
		Log:       logger,
	}
}

func (c *cerebro) Start() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}

	for _, s := range c.Store {
		s.Start(c.Ctx)

		go func(ch <-chan model.Chart) {
			for {
				select {
				case <-c.Ctx.Done():
					break
				case data := <-ch:
					c.ChartData <- data
				}
			}
		}(s.Data())
	}

	go func() {
		for {
			select {
			case data := <-c.ChartData:
				fmt.Println("this")
				fmt.Println(data)
			}
		}
	}()
	return nil
}

func (c *cerebro) AddStore(store store.Storer) {
	c.Store = append(c.Store, store)
}

func (c *cerebro) Stop() {
	c.Cancel()
}
