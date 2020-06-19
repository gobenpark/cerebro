package cerebro

import (
	"trader/broker"
	"trader/store"
)

type Cerebro struct {
	broker broker.Broker
	store  store.Storer
}

func NewCerebro(broker broker.Broker, store store.Storer) *Cerebro {
	return &Cerebro{broker: broker, store: store}
}
