package main

import (
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/cerebro"
	"github.com/BumwooPark/trader/store"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	bk := broker.NewBroker(100000, 0.031)
	cb := cerebro.NewCerebro(bk)

	st := store.NewStore()
	cb.AddStore(st)

	err := cb.Start()
	if err != nil {
		panic(err)
	}

	<-done
}
