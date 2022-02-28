package coin

import (
	"testing"

	"github.com/gobenpark/trader/cerebro"
)

func TestUpbit_Tick(t *testing.T) {
	store := NewStore()
	items := store.GetMarketItems()
	var codes []string
	for _, code := range items {
		codes = append(codes, code.Code)
	}

	c := cerebro.NewCerebro(
		cerebro.WithLive(),
		cerebro.WithStore(NewStore()),
		cerebro.WithTargetItem(codes...),
	)
	c.SetStrategy(st{})
	c.Start()
}
