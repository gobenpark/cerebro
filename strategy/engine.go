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
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
)

type Engine struct {
	log    log.Logger
	store  market.Market
	broker *broker.Broker
	// channels maps an item code to one tick channel per strategy, so Listen can
	// fan a tick out to every strategy instead of letting them steal from a shared one.
	channels map[string][]chan indicator.Tick
	sts      []Strategy
	timeout  time.Duration
	// mu guards channels, which manager writes and Listen reads concurrently.
	mu sync.RWMutex
	// wg tracks the per-strategy Next goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
}

func NewEngine(log log.Logger, bk *broker.Broker, st []Strategy, store market.Market, timeout time.Duration) engine.Engine {
	return &Engine{
		broker:   bk,
		log:      log,
		store:    store,
		timeout:  timeout,
		channels: map[string][]chan indicator.Tick{},
		sts:      st,
	}
}

func (s *Engine) Spawn(ctx context.Context, it []*item.Item) {
	// Drop channels from any previous run; their Next goroutines have exited, so
	// Listen must not keep sending to them (a full buffer would block delivery).
	s.mu.Lock()
	s.channels = map[string][]chan indicator.Tick{}
	s.mu.Unlock()

	for i := range it {
		if err := s.manager(ctx, it[i]); err != nil {
			s.log.Error("manager", "err", err)
			continue
		}
	}
}

func (s *Engine) manager(ctx context.Context, itm *item.Item) error {
	if len(s.sts) == 0 {
		return nil
	}

	// Throttle, then subscribe once per item — the per-strategy channels fan a
	// single market feed out to every strategy, so a second Subscribe would just
	// duplicate ticks. The throttle stays cancellation-aware so shutdown is prompt.
	timer := time.NewTimer(time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
	}

	if err := s.store.Subscribe(func() []*item.Item {
		return []*item.Item{itm}
	}); err != nil {
		return err
	}

	for i := range s.sts {
		// Stop registering further runners once shutdown has started.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Each strategy gets its own channel so Listen can fan a tick out to all
		// of them, instead of strategies stealing from a shared channel.
		ch := make(chan indicator.Tick, 100)
		s.mu.Lock()
		s.channels[itm.Code] = append(s.channels[itm.Code], ch)
		s.mu.Unlock()

		// Next runs until ctx is canceled, so each strategy runs in its own
		// goroutine; otherwise manager would block on the first one.
		s.wg.Go(func() {
			s.sts[i].Next(ctx, itm, ch, s.broker)
		})
	}
	return nil
}

// Wait blocks until every Next goroutine has returned.
func (s *Engine) Wait() {
	s.wg.Wait()
}

func (s *Engine) Listen(ctx context.Context, e any) {
	switch et := e.(type) {
	case order.Order:
		for _, st := range s.sts {
			st.NotifyOrder(et)
		}
	case indicator.Tick:
		s.mu.RLock()
		chs := make([]chan indicator.Tick, len(s.channels[et.Code]))
		copy(chs, s.channels[et.Code])
		s.mu.RUnlock()
		// Fan the tick out to every strategy. Sends are non-blocking: a strategy
		// that isn't keeping up (or has exited during shutdown) drops the tick
		// instead of stalling delivery to the other strategies or the dispatcher.
		for _, c := range chs {
			select {
			case c <- et:
			default:
			}
		}
	}
}
