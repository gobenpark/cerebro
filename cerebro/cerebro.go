package cerebro

import (
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/store"
)

type Cerebroker interface {
	Start()
	AddStore(store.Storer)
}

type cerebro struct {
	broker broker.Broker
	store  []store.Storer
}

func NewCerebro(broker broker.Broker) Cerebroker {
	return &cerebro{broker: broker, store: []store.Storer{}}
}

func (c *cerebro) Start() {

}

func (c *cerebro) AddStore(store store.Storer) {
	c.store = append(c.store, store)
}
