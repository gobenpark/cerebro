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
	"github.com/gobenpark/trader/internal/pkg"
	"github.com/gobenpark/trader/order"
	"go.uber.org/zap"
)

type Engine struct {
	mu      sync.Mutex
	broker  *broker.Broker
	sts     []Strategy
	log     *zap.Logger
	preload bool
}

func NewEngine(log *zap.Logger, bk *broker.Broker, preload bool) *Engine {
	return &Engine{
		broker:  bk,
		log:     log,
		preload: preload,
	}
}

func (s *Engine) AddStrategy(sts ...Strategy) {
	s.sts = append(s.sts, sts...)
}

func (s *Engine) Spawn(ctx context.Context, cont container.Container2, tick <-chan container.Tick) {
	s.log.Info("strategy engine start")

	for i := range pkg.OrDone(ctx, tick) {
		cont.AppendTick(i)
		s.mu.Lock()
		for _, st := range s.sts {
			if err := st.Next(ctx, s.broker, cont); err != nil {
				s.log.Error("next error", zap.Error(err))
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
