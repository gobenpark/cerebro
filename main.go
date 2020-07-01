package main

import (
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/cerebro"
	"github.com/BumwooPark/trader/store"
	"github.com/BumwooPark/trader/strategy"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	bk := broker.NewBroker(100000, 0.031)
	cb := cerebro.NewCerebro(bk)

	st := store.NewAlpaSquareStore()
	cb.AddStore(st)

	smart := strategy.NewSmartStrategy()

	cb.AddStrategy(smart)
	err := cb.Start()
	if err != nil {
		panic(err)
	}

	<-done
}
