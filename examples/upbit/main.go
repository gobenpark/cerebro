package main

import (
	"time"

	"github.com/gobenpark/trader/cerebro"
)

func main() {
	//go func() {
	//	for {
	//		time.Sleep(1 * time.Second)
	//		fmt.Println(runtime.NumGoroutine())
	//	}
	//}()

	upbit := NewStore("upbit")

	smart := &Bighands{}

	cb := cerebro.NewCerebro(
		cerebro.WithStore(upbit, "KRW-MLK"),
		cerebro.WithStrategy(smart),
		cerebro.WithResample("KRW-MFT", time.Minute*3, true),
		cerebro.WithResample("KRW-LBC", time.Minute*3, true),
		cerebro.WithResample("KRW-MLK", time.Minute*3, true),
		cerebro.WithResample("KRW-BTC", time.Minute*3, true),
		cerebro.WithLive(true),
		cerebro.WithPreload(true),
	)

	err := cb.Start()
	if err != nil {
		panic(err)
	}
}
