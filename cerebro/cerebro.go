package cerebro

import (
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/store"
)

type Cerebroker interface {
}

type cerebro struct {
	broker broker.Broker
	store  store.Storer
}

func NewCerebro(broker broker.Broker, store store.Storer) Cerebroker {
	return &cerebro{broker: broker, store: store}
}
