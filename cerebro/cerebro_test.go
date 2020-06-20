package cerebro

import (
	"github.com/BumwooPark/trader/broker"
	"github.com/BumwooPark/trader/store"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func TestCerebro(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	bk := broker.NewBroker(100000, 0.031)
	cerebro := NewCerebro(bk)
	st := store.NewStore()
	cerebro.AddStore(st)
	cerebro.Start()

	<-sigs
	cerebro.Stop()
}
