package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/gobenpark/proto/stock"
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

	cli, err := stock.NewSocketClient(context.Background(), "localhost:50051")
	if err != nil {
		panic(err)
	}

	feed := feeds.NewUpbitFeed("KRW-BTC", cli)
	cb.AddData(feed)
	smart := &strategy.Bighands{
		Broker: bk,
	}
	cb.AddStrategy(smart)
	err = cb.Start()
	if err != nil {
		panic(err)
	}
	<-ch
}
