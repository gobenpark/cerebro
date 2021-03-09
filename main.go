package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/feeds"
	"github.com/gobenpark/trader/strategy"
)

func main() {

	go func() {
		for {
			time.Sleep(1 * time.Second)
			fmt.Println(runtime.NumGoroutine())
		}
	}()
	ch := make(chan bool)

	bk := broker.NewBroker(100000, 0.005)
	cb := cerebro.NewCerebro(bk)

	feed := feeds.NewUpbitFeed()
	cb.AddData(feed)
	smart := &strategy.Bighands{}
	cb.AddStrategy(smart)
	err := cb.Start()
	if err != nil {
		panic(err)
	}
	<-ch
}
