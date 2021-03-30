package main

import (
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/strategy"
)

func main() {
	//go func() {
	//	for {
	//		time.Sleep(1 * time.Second)
	//		fmt.Println(runtime.NumGoroutine())
	//	}
	//}()
	bk := broker.NewBroker(100000, 0.0005)

	upbit := NewStore("upbit")

	smart := &strategy.Bighands{
		Broker: bk,
	}

	cb := cerebro.NewCerebro(
		cerebro.WithBroker(bk),
		cerebro.WithStore(upbit, "KRW-MFT", "KRW-LBC"),
		cerebro.WithStrategy(smart),
		cerebro.WithResample("KRW-MFT", time.Minute*3, true),
		cerebro.WithResample("KRW-LBC", time.Minute*3, true),
		cerebro.WithLive(true),
		cerebro.WithPreload(true),
	)

	err := cb.Start()
	if err != nil {
		panic(err)
	}
}
