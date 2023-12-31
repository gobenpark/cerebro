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
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/store"
)

type Engine struct {
	log         log.Logger
	store       store.Store
	broker      *broker.Broker
	cache       *badger.DB
	eventEngine *event.Engine
	channels    map[string]chan indicator.Tick
	sts         []Strategy
	timeout     time.Duration
	mu          sync.Mutex
}

func NewEngine(log log.Logger, eventEngine *event.Engine, bk *broker.Broker, st []Strategy, store store.Store, cache *badger.DB, timeout time.Duration) engine.Engine {
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

func (s *Engine) Spawn(ctx context.Context, it []item.Item) error {

	filtered := []item.Item{}
	for i := range s.sts {
		for j := range it {
			prd := NewCandleProvider(s.store, it[j])
			if s.sts[i].Pass(it[j], prd) {
				filtered = append(filtered, it[j])
				codech := make(chan indicator.Tick, 1)
				s.channels[it[j].Code] = codech
				cds, err := s.store.Candles(ctx, it[j].Code, store.Day)
				if err != nil {
					s.log.Error("apply candle error", "code", it[j].Code, "err", err)
				}
				v := indicator.NewValue(ctx, cds)
				s.sts[i].Next(it[j], v.Copy(), s.broker)
				v.Start(codech)
			}
		}
	}

	tk, err := s.store.Tick(ctx, filtered...)
	if err != nil {
		s.log.Error("store tick error", "error", err)
		return err
	}

	go func() {
		for i := range tk {
			if c, ok := s.channels[i.Code]; ok {
				select {
				case c <- i:
				case <-ctx.Done():
					break
				}
			}
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
