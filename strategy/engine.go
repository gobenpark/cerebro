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
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/reactivex/rxgo/v2"

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
	cache   *badger.DB
}

func NewEngine(log log.Logger, bk *broker.Broker, preload bool, cache *badger.DB, timeout time.Duration) *Engine {
	return &Engine{
		broker:  bk,
		log:     log,
		preload: preload,
		timeout: timeout,
		cache:   cache,
	}
}

func (s *Engine) AddStrategy(sts ...Strategy) {
	s.sts = append(s.sts, sts...)
}

func (s *Engine) Spawn(ctx context.Context, item []item.Item, observable rxgo.Observable) error {
	s.log.Info("strategy engine start")

	for _, code := range item {
		go func(code string) {
			observable.Filter(func(v interface{}) bool {
				tk := v.(container.Tick)
				return tk.Code == code
			}).Map(func(ctx context.Context, i interface{}) (interface{}, error) {
				tk := i.(container.Tick)
				return container.Tick{
					Code:      tk.Code,
					Price:     tk.Price,
					Volume:    tk.Volume,
					AccVolume: tk.AccVolume,
				}, nil
			}).DoOnNext(func(i interface{}) {
				//tk := i.(container.Tick)
				for _, st := range s.sts {
					st.Next(ctx, s.broker, nil)
				}
			})
		}(code.Code)
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
