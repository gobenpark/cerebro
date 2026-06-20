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
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
)

type Engine struct {
	log         log.Logger
	store       market.Market
	broker      *broker.Broker
	eventEngine *event.Engine
	channels    map[string]chan indicator.Tick
	sts         []Strategy
	timeout     time.Duration
	// mu guards channels, which Spawn writes and Listen reads concurrently.
	mu sync.RWMutex
}

func NewEngine(log log.Logger, eventEngine *event.Engine, bk *broker.Broker, st []Strategy, store market.Market, timeout time.Duration) engine.Engine {
	return &Engine{
		broker:      bk,
		log:         log,
		store:       store,
		timeout:     timeout,
		eventEngine: eventEngine,
		channels:    map[string]chan indicator.Tick{},
		sts:         st,
	}
}

func (s *Engine) Spawn(ctx context.Context, it []*item.Item) {
	for i := range it {
		ch := make(chan indicator.Tick, 100)
		s.mu.Lock()
		s.channels[it[i].Code] = ch
		s.mu.Unlock()

		if err := s.manager(it[i]); err != nil {
			s.log.Error("manager", "err", err)
			continue
		}
	}
}

func (s *Engine) manager(itm *item.Item) error {
	for i := range s.sts {
		time.Sleep(time.Second)
		if err := s.store.Subscribe(func() []*item.Item {
			return []*item.Item{itm}
		}); err != nil {
			return err
		}
		s.mu.RLock()
		ch := s.channels[itm.Code]
		s.mu.RUnlock()
		s.sts[i].Next(itm, ch, s.broker)
	}
	return nil
}

func (s *Engine) Listen(ctx context.Context, e any) {
	switch et := e.(type) {
	case order.Order:
		for _, st := range s.sts {
			st.NotifyOrder(et)
		}
	case indicator.Tick:
		s.mu.RLock()
		c, ok := s.channels[et.Code]
		s.mu.RUnlock()
		if ok {
			select {
			case c <- et:
			case <-ctx.Done():
			}
		}
	}
}
