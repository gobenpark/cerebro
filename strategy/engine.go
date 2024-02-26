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
	mu          sync.Mutex
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

	for i := range s.sts {
		for j := range it {
			codech := make(chan indicator.Tick, 1)
			s.channels[it[j].Code] = codech
			//prd := NewCandleProvider(s.store, it[j])
			//cds, err := s.store.Candles(ctx, it[j].Code, market.Day)
			//if err != nil {
			//	s.log.Error("apply candle error", "code", it[j].Code, "err", err)
			//}
			v := indicator.NewValue(ctx, it[j])
			s.sts[i].Next(it[j], v, nil, s.broker)
			v.Start(codech)
		}
	}
	for i := range s.channels {
		close(s.channels[i])
	}
}

func (s *Engine) Listen(ctx context.Context, e interface{}) {
	switch et := e.(type) {
	case order.Order:
		for _, st := range s.sts {
			st.NotifyOrder(et)
		}
	case indicator.Tick:
		if c, ok := s.channels[et.Code]; ok {
			select {
			case c <- et:
			case <-ctx.Done():
				break
			}
		}
	}
}
