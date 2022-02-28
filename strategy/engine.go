/*
 *  Copyright 2021 The Trader Authors
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

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/log"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	mu sync.Mutex
	broker.Broker
	sts []Strategy
	log log.Logger
}

func NewEngine(log log.Logger, bk broker.Broker) *Engine {

	return &Engine{
		Broker: bk,
		log:    log,
	}
}

func (s *Engine) AddStrategy(sts ...Strategy) {
	s.sts = append(s.sts, sts...)
}

func (s *Engine) Spawn(ctx context.Context, code string, tick <-chan container.Tick) {

	ct := container.NewTradeContainer(code)

	for i := range tick {
		ct.AddTick(i)
		s.mu.Lock()
		for _, st := range s.sts {

			if err := st.Next(s.Broker, ct); err != nil {
				s.log.Error(err)
			}
		}
		s.mu.Unlock()
	}
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
