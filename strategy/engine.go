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

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicators"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/store"
)

type Engine struct {
	mu         sync.Mutex
	broker     *broker.Broker
	sts        []Strategy
	log        log.Logger
	containers []*container
	store      store.Store
	timeout    time.Duration
	cache      *badger.DB
	channels   map[string]chan indicators.Tick
}

func NewEngine(log log.Logger, bk *broker.Broker, preload bool, store store.Store, cache *badger.DB, timeout time.Duration) *Engine {
	return &Engine{
		broker:   bk,
		log:      log,
		store:    store,
		timeout:  timeout,
		cache:    cache,
		channels: map[string]chan indicators.Tick{},
	}
}

func (s *Engine) AddStrategy(sts ...Strategy) {
	s.sts = append(s.sts, sts...)
}

func (s *Engine) Spawn(ctx context.Context, preload bool, item []item.Item) error {
	s.log.Info("strategy engine start")
	tk, err := s.store.Tick(ctx, item...)
	if err != nil {
		s.log.Error("store tick error", "error", err)
		return err
	}

	for _, code := range item {
		s.log.Info("strategy engine spawn", "code", code.Code)
		codech := make(chan indicators.Tick, 1000)
		s.channels[code.Code] = codech

		c := &container{code.Code, s.store, s.cache, indicators.Tick{}}
		go func(code string, ch <-chan indicators.Tick, sts []Strategy, c *container) {
			for i := range ch {
				c.UpdateTick(i)
				for _, st := range s.sts {
					st.Next(ctx, s.broker, c)
				}
			}
		}(code.Code, codech, s.sts, c)
	}

	go func() {
		for i := range tk {
			s.channels[i.Code] <- i
		}

		for i := range s.channels {
			close(s.channels[i])
		}
	}()
	return nil
}

func (s *Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case order.Order:
		for _, st := range s.sts {
			st.NotifyOrder(et)
		}
	case event.CashEvent:
		for _, st := range s.sts {
			st.NotifyCashValue(et.Before, et.After)
		}
	}
}
