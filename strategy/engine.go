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
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
)

// Runner pairs a strategy instance with the universe of items it trades. The
// engine runs one Run goroutine per Runner, so a single-instrument strategy and a
// pairs/portfolio strategy share one execution model — only the size of Items
// differs.
type Runner struct {
	Strategy Strategy
	Items    []*item.Item
}

type Engine struct {
	log    *slog.Logger
	store  market.Market
	broker *broker.Broker
	// channels maps an item code to the tick channels of every running universe that
	// includes that code, so Listen can fan a tick out to each. A portfolio runner's
	// single channel is registered under each of its codes.
	channels map[string][]chan indicator.Tick
	runners  []Runner
	timeout  time.Duration
	// mu guards channels, which launch writes and Listen reads concurrently.
	mu sync.RWMutex
	// wg tracks the per-runner Run goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
}

func NewEngine(log *slog.Logger, bk *broker.Broker, runners []Runner, store market.Market, timeout time.Duration) engine.Engine {
	return &Engine{
		broker:   bk,
		log:      log,
		store:    store,
		timeout:  timeout,
		channels: map[string][]chan indicator.Tick{},
		runners:  runners,
	}
}

// universe is the engine's Universe implementation handed to each Run.
type universe struct {
	items []*item.Item
	ticks <-chan indicator.Tick
}

func (u *universe) Items() []*item.Item          { return u.items }
func (u *universe) Ticks() <-chan indicator.Tick { return u.ticks }

func (s *Engine) Spawn(ctx context.Context) {
	// Register EVERY runner's tick channel up front — under each code in its
	// universe — BEFORE subscribing, and collect the union of items. A market starts
	// a code's feed when it is subscribed, so registering all channels first
	// guarantees that when a code begins emitting, every runner that trades it
	// already has somewhere to receive its ticks. This matters when several runners
	// share a code (two WithStrategy registrations over the same target, or a
	// portfolio strategy alongside a per-item one): registering and subscribing
	// per-runner would let a later runner miss the ticks emitted before it
	// registered. A fresh map also drops channels from a previous run whose Run
	// goroutines have exited.
	type active struct {
		r  Runner
		ch chan indicator.Tick
	}
	var (
		actives []active
		union   []*item.Item
		seen    = map[string]struct{}{}
	)

	s.mu.Lock()
	s.channels = map[string][]chan indicator.Tick{}
	for i := range s.runners {
		r := s.runners[i]
		if len(r.Items) == 0 {
			continue
		}
		// The buffer scales with the universe size.
		ch := make(chan indicator.Tick, 100*len(r.Items))
		for _, itm := range r.Items {
			s.channels[itm.Code] = append(s.channels[itm.Code], ch)
			if _, ok := seen[itm.Code]; !ok {
				seen[itm.Code] = struct{}{}
				union = append(union, itm)
			}
		}
		actives = append(actives, active{r: r, ch: ch})
	}
	s.mu.Unlock()

	if len(actives) == 0 || ctx.Err() != nil {
		s.resetChannels()
		return
	}

	// Subscribe the whole universe in one call, then start every runner together.
	// One subscribe (rather than one per runner) means later runners are not
	// staggered behind earlier ones, so a short backtest's feed does not finish
	// before they start. If the market rejects the subscription, start nothing and
	// leave no channels registered.
	if err := s.store.Subscribe(func() []*item.Item { return union }); err != nil {
		s.log.Error("subscribe", "err", err)
		s.resetChannels()
		return
	}

	for _, a := range actives {
		// Hand each strategy a broker scoped to its name so its orders are attributed.
		bk := s.broker.Scoped(a.r.Strategy.Name())
		u := &universe{items: a.r.Items, ticks: a.ch}
		st := a.r.Strategy
		// Run blocks until ctx is canceled, so it runs in its own goroutine.
		s.wg.Go(func() {
			st.Run(ctx, u, bk)
		})
	}
}

// resetChannels clears the listener channel map so no tick is delivered to a
// runner that did not start (e.g. after a failed subscribe).
func (s *Engine) resetChannels() {
	s.mu.Lock()
	s.channels = map[string][]chan indicator.Tick{}
	s.mu.Unlock()
}

// Wait blocks until every Run goroutine has returned.
func (s *Engine) Wait() {
	s.wg.Wait()
}

func (s *Engine) Listen(ctx context.Context, e any) {
	switch et := e.(type) {
	case order.Order:
		// Route the order update to its owning strategy only. Unattributed orders
		// (no strategy tag) go to every strategy, preserving prior behavior. Runner
		// names are unique (enforced at startup), so an attributed order reaches
		// exactly one runner.
		owner := et.Strategy()
		for i := range s.runners {
			if owner == "" || s.runners[i].Strategy.Name() == owner {
				s.runners[i].Strategy.NotifyOrder(et)
			}
		}
	case indicator.Tick:
		s.mu.RLock()
		chs := slices.Clone(s.channels[et.Code])
		s.mu.RUnlock()
		// Fan the tick out to every runner subscribed to its code. Sends are
		// non-blocking: a runner that isn't keeping up (or has exited during
		// shutdown) drops the tick instead of stalling delivery to the others or the
		// dispatcher.
		for _, c := range chs {
			select {
			case c <- et:
			default:
			}
		}
	}
}
