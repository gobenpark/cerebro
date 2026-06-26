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
	"fmt"
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

var _ engine.Engine = (*Engine)(nil)

// runnerHandle tracks one dynamically added runner so RemoveRunner can tear exactly
// it down: its per-code channels (to unregister from the fanout maps) and the cancel
// that stops its Run goroutine.
type runnerHandle struct {
	runner Runner
	tickCh chan indicator.Tick
	obCh   chan indicator.OrderBook
	exCh   chan any
	cancel context.CancelFunc
}

type Engine struct {
	log    *slog.Logger
	store  market.Market
	broker *broker.Broker
	// channels maps an item code to the tick channels of every running universe that
	// includes that code, so Listen can fan a tick out to each. A portfolio runner's
	// single channel is registered under each of its codes.
	channels map[string][]chan indicator.Tick
	// obChannels mirrors channels for order-book (호가) updates: an item code maps to
	// the order-book channel of every running universe that includes it.
	obChannels map[string][]chan indicator.OrderBook
	// extraChannels mirrors channels for adapter-specific Extras events: an item code
	// maps to the Extras channel of every running universe that includes it. A
	// code-less Extras event (one not implementing Coded) fans out to every channel
	// here, deduplicated.
	extraChannels map[string][]chan any
	runners       []Runner
	timeout       time.Duration
	// dynamic holds runners added at runtime via AddRunner (a screener-driven
	// watchlist), keyed by strategy name, so RemoveRunner can stop exactly one. The
	// runners from Spawn are permanent and are not tracked here.
	dynamic map[string]*runnerHandle
	// mu guards channels, obChannels, runners, and dynamic, which launch/reconcile
	// writes and Listen reads concurrently.
	mu sync.RWMutex
	// wg tracks the per-runner Run goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
}

// NewEngine returns the concrete *Engine. It satisfies engine.Engine for the event
// dispatcher, and its AddRunner/RemoveRunner expose the runtime runner lifecycle a
// screener-driven watchlist reconciles against. Spawn's runners are permanent; only
// AddRunner's can be removed.
func NewEngine(log *slog.Logger, bk *broker.Broker, runners []Runner, store market.Market, timeout time.Duration) *Engine {
	return &Engine{
		broker:        bk,
		log:           log,
		store:         store,
		timeout:       timeout,
		channels:      map[string][]chan indicator.Tick{},
		obChannels:    map[string][]chan indicator.OrderBook{},
		extraChannels: map[string][]chan any{},
		dynamic:       map[string]*runnerHandle{},
		runners:       runners,
	}
}

// universe is the engine's Universe implementation handed to each Run.
type universe struct {
	items      []*item.Item
	ticks      <-chan indicator.Tick
	orderbooks <-chan indicator.OrderBook
	extras     <-chan any

	// market sources historical candles for Warmup. cmu guards the lazily-started
	// candle dispatcher's state (see warmup.go): streams holds the per-code seeded
	// resamplers, started marks the single tick-consuming goroutine as running, and
	// pending buffers ticks for a code whose stream is not registered yet (a later
	// Warmup in a multi-code strategy) so it catches up instead of missing bars.
	market  market.Market
	cmu     sync.Mutex
	streams map[string][]*candleStream
	pending map[string][]indicator.Tick
	started bool
}

func (u *universe) Items() []*item.Item                    { return u.items }
func (u *universe) Ticks() <-chan indicator.Tick           { return u.ticks }
func (u *universe) OrderBooks() <-chan indicator.OrderBook { return u.orderbooks }
func (u *universe) Extras() <-chan any                     { return u.extras }

func (s *Engine) Spawn(ctx context.Context) {
	// Register EVERY runner's tick channel up front — under each code in its
	// universe — BEFORE subscribing, and collect the union of items. A market starts
	// a code's feed when it is subscribed, so registering all channels first
	// guarantees that when a code begins emitting, every runner that trades it
	// already has somewhere to receive its ticks. This matters when several runners
	// share a code (two WithStrategy registrations over the same watchlist, or a
	// portfolio strategy alongside a per-item one): registering and subscribing
	// per-runner would let a later runner miss the ticks emitted before it
	// registered. A fresh map also drops channels from a previous run whose Run
	// goroutines have exited.
	type active struct {
		r    Runner
		ch   chan indicator.Tick
		obch chan indicator.OrderBook
		exch chan any
	}
	var (
		actives []active
		union   []*item.Item
		seen    = map[string]struct{}{}
	)

	s.mu.Lock()
	s.channels = map[string][]chan indicator.Tick{}
	s.obChannels = map[string][]chan indicator.OrderBook{}
	s.extraChannels = map[string][]chan any{}
	for i := range s.runners {
		r := s.runners[i]
		if len(r.Items) == 0 {
			continue
		}
		// The buffers scale with the universe size.
		ch := make(chan indicator.Tick, 100*len(r.Items))
		obch := make(chan indicator.OrderBook, 100*len(r.Items))
		exch := make(chan any, 100*len(r.Items))
		for _, itm := range r.Items {
			s.channels[itm.Code] = append(s.channels[itm.Code], ch)
			s.obChannels[itm.Code] = append(s.obChannels[itm.Code], obch)
			s.extraChannels[itm.Code] = append(s.extraChannels[itm.Code], exch)
			if _, ok := seen[itm.Code]; !ok {
				seen[itm.Code] = struct{}{}
				union = append(union, itm)
			}
		}
		actives = append(actives, active{r: r, ch: ch, obch: obch, exch: exch})
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
	if err := s.store.Subscribe(ctx, func() []*item.Item { return union }); err != nil {
		s.log.Error("subscribe", "err", err)
		s.resetChannels()
		return
	}

	for _, a := range actives {
		// Hand each strategy a broker scoped to its name so its orders are attributed.
		bk := s.broker.Scoped(a.r.Strategy.Name())
		u := &universe{items: a.r.Items, ticks: a.ch, orderbooks: a.obch, extras: a.exch, market: s.store}
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
	s.obChannels = map[string][]chan indicator.OrderBook{}
	s.extraChannels = map[string][]chan any{}
	s.mu.Unlock()
}

// AddRunner spawns one runner at runtime, after Spawn — the path a screener-driven
// watchlist uses to start a strategy as an item qualifies. It registers the runner's
// per-code channels before subscribing (so the first ticks are not missed),
// subscribes its codes incrementally, and launches Run under a per-runner context
// derived from ctx so RemoveRunner can stop just this one. A runner with no items is
// a no-op; a name already running is an error. On a subscribe failure it rolls back,
// leaving nothing registered.
func (s *Engine) AddRunner(ctx context.Context, r Runner) error {
	if len(r.Items) == 0 {
		return nil
	}
	name := r.Strategy.Name()

	s.mu.Lock()
	if _, ok := s.dynamic[name]; ok {
		s.mu.Unlock()
		return fmt.Errorf("strategy: runner %q already running", name)
	}
	// A dynamic runner must not collide with a permanent (Spawn) runner's name, or an
	// attributed order would route to both and their ledgers would conflate.
	for i := range s.runners {
		if s.runners[i].Strategy.Name() == name {
			s.mu.Unlock()
			return fmt.Errorf("strategy: runner %q collides with a permanent runner", name)
		}
	}
	tickCh := make(chan indicator.Tick, 100*len(r.Items))
	obCh := make(chan indicator.OrderBook, 100*len(r.Items))
	exCh := make(chan any, 100*len(r.Items))
	for _, itm := range r.Items {
		s.channels[itm.Code] = append(s.channels[itm.Code], tickCh)
		s.obChannels[itm.Code] = append(s.obChannels[itm.Code], obCh)
		s.extraChannels[itm.Code] = append(s.extraChannels[itm.Code], exCh)
	}
	rctx, cancel := context.WithCancel(ctx)
	h := &runnerHandle{runner: r, tickCh: tickCh, obCh: obCh, exCh: exCh, cancel: cancel}
	s.dynamic[name] = h
	s.mu.Unlock()

	// Register-before-subscribe holds: the channels are already in the maps, so a tick
	// arriving the moment the feed opens is routed rather than dropped.
	if err := s.store.Subscribe(rctx, func() []*item.Item { return r.Items }); err != nil {
		cancel()
		s.mu.Lock()
		s.removeChannelsLocked(h)
		delete(s.dynamic, name)
		s.mu.Unlock()
		return fmt.Errorf("strategy: subscribe %q: %w", name, err)
	}

	bk := s.broker.Scoped(name)
	u := &universe{items: r.Items, ticks: tickCh, orderbooks: obCh, extras: exCh, market: s.store}
	st := r.Strategy
	// Run blocks until rctx is canceled (by RemoveRunner or shutdown), so it runs in
	// its own goroutine.
	s.wg.Go(func() {
		st.Run(rctx, u, bk)
	})
	return nil
}

// RemoveRunner stops a runner added by AddRunner: it unregisters the runner's
// channels from the fanout maps and cancels its Run (which returns on ctx.Done). The
// channels are left to be garbage-collected rather than closed, so an in-flight
// Listen fanout never sends on a closed channel. If the market adapter implements
// market.Unsubscriber, the codes' feed is released; otherwise routing simply stops.
// Removing an unknown, already-removed, or Spawn-permanent runner is a no-op.
func (s *Engine) RemoveRunner(ctx context.Context, name string) {
	s.mu.Lock()
	h, ok := s.dynamic[name]
	if !ok {
		s.mu.Unlock()
		return
	}
	s.removeChannelsLocked(h)
	delete(s.dynamic, name)
	// Only codes no longer routed to ANY runner are safe to unsubscribe;
	// removeChannelsLocked deletes a code's entry when its last channel is gone, so an
	// absent entry means no remaining user (dynamic or permanent). A code another live
	// runner still trades must keep its feed.
	var orphan []string
	for _, it := range h.runner.Items {
		if len(s.channels[it.Code]) == 0 {
			orphan = append(orphan, it.Code)
		}
	}
	s.mu.Unlock()

	h.cancel()

	if u, ok := s.store.(market.Unsubscriber); ok && len(orphan) > 0 {
		if err := u.Unsubscribe(ctx, orphan); err != nil {
			s.log.Info("unsubscribe", "strategy", name, "err", err)
		}
	}
}

// removeChannelsLocked drops h's channels from the per-code fanout maps by identity.
// Caller holds s.mu.
func (s *Engine) removeChannelsLocked(h *runnerHandle) {
	for _, itm := range h.runner.Items {
		s.channels[itm.Code] = slices.DeleteFunc(s.channels[itm.Code], func(c chan indicator.Tick) bool {
			return c == h.tickCh
		})
		if len(s.channels[itm.Code]) == 0 {
			delete(s.channels, itm.Code)
		}
		s.obChannels[itm.Code] = slices.DeleteFunc(s.obChannels[itm.Code], func(c chan indicator.OrderBook) bool {
			return c == h.obCh
		})
		if len(s.obChannels[itm.Code]) == 0 {
			delete(s.obChannels, itm.Code)
		}
		s.extraChannels[itm.Code] = slices.DeleteFunc(s.extraChannels[itm.Code], func(c chan any) bool {
			return c == h.exCh
		})
		if len(s.extraChannels[itm.Code]) == 0 {
			delete(s.extraChannels, itm.Code)
		}
	}
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
		// exactly one runner. Collect targets under the lock (dynamic runners are added
		// and removed concurrently), then notify outside it so user code never runs
		// while the lock is held.
		owner := et.Strategy()
		s.mu.RLock()
		targets := make([]Strategy, 0, len(s.runners)+len(s.dynamic))
		for i := range s.runners {
			if owner == "" || s.runners[i].Strategy.Name() == owner {
				targets = append(targets, s.runners[i].Strategy)
			}
		}
		for _, h := range s.dynamic {
			if owner == "" || h.runner.Strategy.Name() == owner {
				targets = append(targets, h.runner.Strategy)
			}
		}
		s.mu.RUnlock()
		for _, st := range targets {
			st.NotifyOrder(et)
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
	case indicator.OrderBook:
		s.mu.RLock()
		chs := slices.Clone(s.obChannels[et.Code])
		s.mu.RUnlock()
		// Fan the order-book snapshot out to every runner subscribed to its code, with
		// the same non-blocking, drop-if-behind delivery as ticks.
		for _, c := range chs {
			select {
			case c <- et:
			default:
			}
		}
	case market.ChangeOrderEvent, market.ChangeBalanceEvent, market.FeedStatusEvent:
		// cerebro-internal events — order/balance changes are the broker's, feed-status
		// the watchdog's. None is strategy-facing data, so none leaks to Extras.
	default:
		// Any other event a market emitted is adapter-specific (e.g. program-trade flow):
		// fan it out to the subscribed universes' Extras streams.
		s.fanoutExtra(e)
	}
}

// fanoutExtra delivers an adapter-specific Extras event to the universes that should
// see it: those subscribed to e.Code() when e implements Coded, else every Extras
// channel (deduplicated — a portfolio universe registers one channel under several
// codes). Best-effort like ticks: a universe behind on Extras drops the event rather
// than stalling the dispatcher.
func (s *Engine) fanoutExtra(e any) {
	s.mu.RLock()
	var chs []chan any
	if c, ok := e.(Coded); ok {
		chs = slices.Clone(s.extraChannels[c.Code()])
	} else {
		seen := make(map[chan any]struct{})
		for _, list := range s.extraChannels {
			for _, c := range list {
				if _, ok := seen[c]; !ok {
					seen[c] = struct{}{}
					chs = append(chs, c)
				}
			}
		}
	}
	s.mu.RUnlock()
	for _, c := range chs {
		select {
		case c <- e:
		default:
		}
	}
}
