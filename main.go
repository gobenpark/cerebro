package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/feeds"
	store2 "github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

func main() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			fmt.Println(runtime.NumGoroutine())
		}
	}()
	bk := broker.NewBroker(100000, 0.005)
	store := store2.NewStore()

	feed := feeds.NewFeed("KRW-BTC", store)
	smart := &strategy.Bighands{
		Broker: bk,
	}
	cb := cerebro.NewCerebro(
		cerebro.WithBroker(bk),
		cerebro.WithStore(store),
		cerebro.WithFeed(feed),
		cerebro.WithStrategy(smart),
	)

	err := cb.Start()
	if err != nil {
		panic(err)
	}
}
