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
	"time"

	"github.com/gobenpark/cerebro/log"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/container"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/order"
)

type Engine struct {
	mu      sync.Mutex
	broker  *broker.Broker
	sts     []Strategy
	log     log.Logger
	preload bool
	chs     []chan container.Container
	timeout time.Duration
}

func NewEngine(log log.Logger, bk *broker.Broker, preload bool, timeout time.Duration) *Engine {
	return &Engine{
		broker:  bk,
		log:     log,
		preload: preload,
		timeout: timeout,
	}
}

func (s *Engine) AddStrategy(sts ...Strategy) {
	s.sts = append(s.sts, sts...)
}

func (s *Engine) Spawn(ctx context.Context, cont <-chan container.Container) error {
	s.log.Info("strategy engine start")

	for _, i := range s.sts {
		ch := make(chan container.Container, 1)
		s.chs = append(s.chs, ch)
		go func(st Strategy, c <-chan container.Container) {
			for j := range c {
				s.log.Debug("receive event", "strategy", st.Name())
				if s.timeout == 0 {
					ctx, cancel := context.WithTimeout(ctx, s.timeout)
					if err := st.Next(ctx, s.broker, j); err != nil {
						s.log.Error("error strategy engine", "err", err)
						continue
					}
					cancel()
					continue
				}

				if err := st.Next(ctx, s.broker, j); err != nil {
					s.log.Error("error strategy engine", "err", err)
				}
			}
		}(i, ch)
	}

	for i := range cont {
		for _, j := range s.chs {
			j <- i
		}
	}

	for i := range s.sts {
		close(s.chs[i])
	}
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
