/*
 *  Copyright 2021 The Cerebro Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
package strategy

import (
	"context"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/cache"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/samber/lo"
)

type Engine struct {
	log         log.Logger
	store       market.Market
	broker      *broker.Broker
	cache       *cache.Cache
	eventEngine *event.Engine
	channels    map[string]chan indicator.Tick
	sts         []Strategy
	timeout     time.Duration
	mu          sync.Mutex
}

func NewEngine(log log.Logger, eventEngine *event.Engine, bk *broker.Broker, st []Strategy, store market.Market, cache *cache.Cache, timeout time.Duration) engine.Engine {
	return &Engine{
		broker:      bk,
		log:         log,
		store:       store,
		timeout:     timeout,
		cache:       cache,
		eventEngine: eventEngine,
		channels:    map[string]chan indicator.Tick{},
		sts:         st,
	}
}

func (s *Engine) Spawn(ctx context.Context, it []*item.Item, tk <-chan indicator.Tick) error {

	for i := range s.sts {
		for j := range it {
			codech := make(chan indicator.Tick, 1)
			s.channels[it[j].Code] = codech
			//prd := NewCandleProvider(s.store, it[j])
			//cds, err := s.store.Candles(ctx, it[j].Code, market.Day)
			//if err != nil {
			//	s.log.Error("apply candle error", "code", it[j].Code, "err", err)
			//}
			v := indicator.NewValue(ctx, nil)
			s.sts[i].Next(it[j], v, nil, s.broker)
			v.Start(codech)
		}
	}

Done:
	for i := range tk {
		t, ok := lo.Find[*item.Item](it, func(item *item.Item) bool {
			return item.Code == i.Code
		})

		if ok {
			for i := range s.sts {
				s.sts[i].Estimation(t, NewCandleProvider(s.store, t))
			}
		}

		if c, ok := s.channels[i.Code]; ok {
			if t.Status() != item.Unactivate {
				continue
			}
			select {
			case c <- i:
			case <-ctx.Done():
				break Done
			}
		}
	}
	for i := range s.channels {
		close(s.channels[i])
	}
	return nil
}

func (s *Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case order.Order:
		for _, st := range s.sts {
			st.NotifyOrder(et)
		}
	}
}
