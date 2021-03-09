package main

import (
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/strategy"
)

func main() {

	bk := broker.NewBroker(100000, 0.005)
	cb := cerebro.NewCerebro(bk)

	//st := store.NewAlpaSquareStore()
	//cb.AddStore(st)

	smart := &strategy.Bighands{}
	cb.AddStrategy(smart)
	err := cb.Start()
	if err != nil {
		panic(err)
	}
}
