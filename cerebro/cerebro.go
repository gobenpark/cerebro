package cerebro

import (
	"context"
	"fmt"
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/store"
	"github.com/BumwooPark/trader/store/model"
	"go.uber.org/zap"
)

type Cerebroker interface {
	Start() error
	Stop()
	AddStore(store.Storer)
}

type cerebro struct {
	broker broker.Broker
	store  []store.Storer
	ctx    context.Context
	cancel context.CancelFunc
	log    *zap.Logger
}

func NewCerebro(broker broker.Broker) Cerebroker {
	ctx, cancel := context.WithCancel(context.Background())
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	return &cerebro{
		broker: broker,
		store:  []store.Storer{},
		ctx:    ctx,
		cancel: cancel,
		log:    logger,
	}
}

func (c *cerebro) Start() error {

	for _, s := range c.store {
		s.Start(c.ctx)

		go func(ch <-chan model.Chart) {
			for {
				select {
				case <-c.ctx.Done():
					break
				case data := <-ch:
					fmt.Println(data)
				}
			}
		}(s.Data())
	}
	return nil
}

func (c *cerebro) AddStore(store store.Storer) {
	c.store = append(c.store, store)
}

func (c *cerebro) Stop() {
	c.cancel()
}
