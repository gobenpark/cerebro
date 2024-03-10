package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/market"
	"github.com/maypok86/otter"
)

type Manager struct {
	cache otter.Cache[string, indicator.Candles]
	store market.Market
}

func NewManager(store market.Market) *Manager {
	cache, err := otter.MustBuilder[string, indicator.Candles](10_000).
		CollectStats().
		Cost(func(key string, value indicator.Candles) uint32 {
			return 1
		}).
		Build()
	if err != nil {
		panic(err)
	}
	return &Manager{
		cache: cache,
		store: store,
	}
}

func (m *Manager) Calculate(tk indicator.Tick) {
	v, ok := m.cache.Get(fmt.Sprintf("%s:day", tk.Code))
	if !ok {
		cds, err := m.store.Candles(context.TODO(), tk.Code, market.Day)
		if err != nil {
			fmt.Println(err)
			return
		}
		data := indicator.Resampler(cds, tk, 24*time.Hour)
		fmt.Println(data[data.Len()-1])
		m.cache.Set(fmt.Sprintf("%s:day", tk.Code), data)
		//fmt.Println(cds)
		return
	}
	m.cache.Set(fmt.Sprintf("%s:day", tk.Code), indicator.Resampler(v, tk, 24*time.Hour))
	fmt.Println(v[v.Len()-1])
}
