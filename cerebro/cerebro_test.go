package cerebro

import (
	"fmt"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/store"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestCerebro(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	bk := broker.NewBroker(100000, 0.015)
	cerebro := NewCerebro(bk)
	st := store.NewStore()
	cerebro.AddStore(st)
	err := cerebro.Start()
	if err != nil {
		fmt.Println(err)
	}

	<-sigs
	cerebro.Stop()
}
