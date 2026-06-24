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
package cerebro

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	cancel   context.CancelFunc
	logLevel slog.Level
	// broker buy, sell and manage order
	broker *broker.Broker

	target []*item.Item

	// screener, when set, supplies the watchlist dynamically at Start by merging
	// its picks into target (the dynamic counterpart of WithTargetItem).
	screener Screener
	// started guards Start: the screener merge mutates target, so Start is a
	// one-shot — a repeat call is rejected rather than re-merging into stale state.
	started bool

	market market.Market

	log *slog.Logger

	// eventEngine engine of management all event
	eventEngine *event.Engine

	// stratRegs are explicit-universe strategy registrations (WithStrategy): one
	// instance trading a fixed set of codes (or all targets when none are given).
	stratRegs []stratReg
	// forEachRegs are per-item registrations (WithStrategyForEach): a fresh instance
	// per target item, each with a single-item universe.
	forEachRegs []forEachReg

	// risk is the optional pre-trade gate; installed on the broker when set.
	risk *risk.Manager

	// storage is the optional durable ledger store; installed on the broker when
	// set so per-strategy PnL/fees/lots survive a restart.
	storage broker.Storage

	// policies maps a strategy name to its reactive exit policy (WithRiskPolicy).
	policies map[string]risk.Policy
	// monitor evaluates the policies against attributed fills; nil when none are set.
	monitor *risk.Monitor

	timeout time.Duration

	// feedTimeout, when > 0, arms a market-data staleness watchdog in the events
	// pump: a tick (or market.FeedStatusEvent) resets it, and if it elapses with no
	// such event the feed is treated as lost. Zero disables it (suits backtests).
	feedTimeout time.Duration
	// feedLossHandler runs when the feed is lost (stale, or its channel closes while
	// the run is still live). When nil and feed guarding is active, the default is a
	// fail-safe Shutdown so the engine does not trade on a dead feed.
	feedLossHandler func(reason string)

	engines []engine.Engine
	// wg tracks the producer goroutines (spawn, market events) started by Start.
	wg sync.WaitGroup
	// eventCancel stops the event dispatcher; it runs on its own context so the
	// dispatcher can outlive producers and be torn down last during shutdown.
	eventCancel context.CancelFunc
	// eventWg tracks the event dispatcher goroutine. The dispatcher waits for its
	// per-listener workers to drain before returning, so joining eventWg guarantees
	// every dispatched event has been processed.
	eventWg sync.WaitGroup
	// shutdownOnce makes Shutdown idempotent; it may be triggered both explicitly
	// and by parent-context cancellation.
	shutdownOnce sync.Once
}

// NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	c := &Cerebro{
		eventEngine: event.NewEventEngine(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.log == nil {
		// Default to a structured stderr logger at the configured level (Info by
		// default). Inject your own with WithLogger to route Cerebro's logs into an
		// existing slog pipeline. logLevel's zero value is slog.LevelInfo.
		c.log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     c.logLevel,
		}))
	}

	c.broker = broker.NewDefaultBroker(c.eventEngine, c.market, c.log)
	if c.risk != nil {
		c.broker.SetRisk(c.risk)
	}
	if c.storage != nil {
		c.broker.SetStorage(c.storage)
	}
	// Build a reactive monitor for any configured exit policy. Empty policies (no
	// trigger set) are dropped so a misconfigured option is inert rather than fatal.
	active := map[string]risk.Policy{}
	for name, p := range c.policies {
		if p.Enabled() {
			active[name] = p
		}
	}
	if len(active) > 0 {
		c.monitor = risk.NewMonitor(c.log, active, func(name string) risk.Submitter {
			return c.broker.Scoped(name)
		}, c.broker)
	}
	// The strategy engine is built in Start, once target items are known, since
	// resolving registrations into runners needs them and can fail (e.g. a strategy
	// references an unknown code).

	return c
}

// stratReg is one WithStrategy registration: a strategy instance and the codes it
// trades. An empty codes slice means "the whole target set".
type stratReg struct {
	s     strategy.Strategy
	codes []string
}

// forEachReg is one WithStrategyForEach registration: a factory that produces a
// fresh strategy instance for each target item.
type forEachReg struct {
	factory func(*item.Item) strategy.Strategy
}

// resolveRunners turns the strategy registrations into the flat list of runners
// the engine executes, validating that referenced codes exist and that every
// runner's name is unique (so order notifications route to exactly one runner).
func (c *Cerebro) resolveRunners(target []*item.Item) ([]strategy.Runner, error) {
	byCode := make(map[string]*item.Item, len(target))
	for _, it := range target {
		byCode[it.Code] = it
	}

	var runners []strategy.Runner
	for _, reg := range c.stratRegs {
		items := target
		if len(reg.codes) > 0 {
			items = make([]*item.Item, 0, len(reg.codes))
			seenCode := make(map[string]struct{}, len(reg.codes))
			for _, code := range reg.codes {
				if _, dup := seenCode[code]; dup {
					// A repeated code would register the runner's tick channel twice
					// under that code, delivering every tick to the strategy twice.
					return nil, fmt.Errorf("strategy %q lists duplicate code %q", reg.s.Name(), code)
				}
				seenCode[code] = struct{}{}
				it, ok := byCode[code]
				if !ok {
					return nil, fmt.Errorf("strategy %q references unknown target code %q", reg.s.Name(), code)
				}
				items = append(items, it)
			}
		}
		runners = append(runners, strategy.Runner{Strategy: reg.s, Items: items})
	}
	for _, reg := range c.forEachRegs {
		for _, it := range target {
			runners = append(runners, strategy.Runner{Strategy: reg.factory(it), Items: []*item.Item{it}})
		}
	}

	seen := make(map[string]struct{}, len(runners))
	for _, r := range runners {
		name := r.Strategy.Name()
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("duplicate strategy name %q (names must be unique across strategies and per-item instances)", name)
		}
		seen[name] = struct{}{}
	}
	return runners, nil
}

// resolveTarget builds the effective watchlist for this run: a clone of the
// configured target merged (deduped by code) with any screener picks. Cloning keeps
// Start retryable — a failure never mutates c.target. A configured screener that
// returns nothing is reported distinctly from a missing-target error, since an empty
// screen (no name qualified today) is a common, expected case rather than misconfig.
func (c *Cerebro) resolveTarget(ctx context.Context) ([]*item.Item, error) {
	target := slices.Clone(c.target)
	if c.screener != nil {
		items, err := c.screener.Screen(ctx)
		if err != nil {
			return nil, fmt.Errorf("screener: %w", err)
		}
		seen := make(map[string]struct{}, len(target))
		for _, it := range target {
			seen[it.Code] = struct{}{}
		}
		for _, it := range items {
			if _, ok := seen[it.Code]; ok {
				continue
			}
			seen[it.Code] = struct{}{}
			target = append(target, it)
		}
		if len(target) == 0 {
			return nil, fmt.Errorf("screener returned an empty watchlist and no target was set")
		}
	}
	if len(target) == 0 {
		return nil, fmt.Errorf("no target items set")
	}
	return target, nil
}

// validatePolicies rejects an enabled exit policy that names a strategy no runner
// provides: its exits could never trigger (no fills would be attributed to an unknown
// name), so a typo must fail fast. Disabled (empty) policies are inert and skipped.
func (c *Cerebro) validatePolicies(runners []strategy.Runner) error {
	if len(c.policies) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(runners))
	for _, r := range runners {
		known[r.Strategy.Name()] = struct{}{}
	}
	for name, p := range c.policies {
		if !p.Enabled() {
			continue
		}
		if _, ok := known[name]; !ok {
			return fmt.Errorf("risk policy references unknown strategy %q", name)
		}
	}
	return nil
}

// Start run cerebro. It is one-shot: the screener merge mutates target, so a
// second call is rejected.
func (c *Cerebro) Start(ctx context.Context) error {
	if c.started {
		return fmt.Errorf("cerebro: Start called more than once")
	}

	// Resolve and validate the run configuration before spawning anything, so a bad
	// config fails fast and leaks no goroutines.
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return err
	}
	if len(c.stratRegs) == 0 && len(c.forEachRegs) == 0 {
		return fmt.Errorf("no strategies registered")
	}
	// resolveRunners validates referenced codes and name uniqueness; validatePolicies
	// rejects an exit policy that names a strategy no runner provides.
	runners, err := c.resolveRunners(target)
	if err != nil {
		return err
	}
	if err := c.validatePolicies(runners); err != nil {
		return err
	}
	// Restore the persisted ledger (per-strategy PnL/fees/open lots) as the final
	// fallible step — before any context or goroutine is created — so a restore
	// failure returns with nothing to tear down and leaves the instance retryable.
	// The broker is populated before listeners go live, so the first fill event sees it.
	if err := c.broker.Restore(ctx); err != nil {
		return fmt.Errorf("restore broker ledger: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// The dispatcher runs on its own context so it can keep draining broadcasts
	// until every producer (spawn, events, strategies, broker) has stopped. It is
	// detached from parent cancellation and stopped only by Shutdown.
	eventCtx, eventCancel := context.WithCancel(context.WithoutCancel(ctx))
	c.eventCancel = eventCancel

	c.log.Info("Cerebro starting ...")

	// Build the strategy engine only after every fallible step has succeeded, and
	// assign (not append) so retrying Start after an earlier failure does not leave
	// a stale engine behind that would double-register and double-spawn.
	c.engines = []engine.Engine{strategy.NewEngine(c.log, c.broker, runners, c.market, c.timeout)}

	c.markHeldItemsActive(ctx, target)

	// Every fallible step has succeeded: commit the resolved watchlist and latch
	// the one-shot guard. A failure before here leaves the instance retryable
	// (c.target and c.started unchanged).
	c.target = target
	c.started = true

	c.eventWg.Go(func() {
		c.eventEngine.Start(eventCtx)
	})

	// Register every listener synchronously BEFORE the producers run, then spawn the
	// strategy producers and the market-events pump.
	c.registerListeners(ctx)
	c.spawnStrategies(ctx)
	c.wg.Go(func() {
		c.pumpEvents(ctx, eventCtx)
	})

	// Register the cancellation hook only after every goroutine and shutdown field
	// exists, so an already-canceled context can't consume shutdownOnce before the
	// producers/dispatcher are set up. Canceling the parent context now triggers a
	// graceful, ordered shutdown.
	context.AfterFunc(ctx, c.Shutdown)

	return nil
}

// Report returns a per-strategy snapshot of realized PnL, fees, and open
// positions, built from attributed fills. It is safe to call at any time and is
// handy to print at the end of a run.
func (c *Cerebro) Report() []broker.StrategyReport {
	return c.broker.Report()
}

// feedGuarded reports whether live-feed guarding is active — either a staleness
// watchdog is armed (WithFeedTimeout) or a feed-loss handler is installed
// (WithFeedLossHandler). When false (the default) the events pump behaves as before:
// a channel close is the normal end of the stream, not a feed loss.
func (c *Cerebro) feedGuarded() bool {
	return c.feedTimeout > 0 || c.feedLossHandler != nil
}

// onFeedLoss handles a degraded market feed — it went stale (no data within the feed
// timeout) or its channel closed while the run was still live. It runs the configured
// handler, or, by default, a fail-safe Shutdown so the engine does not keep trading
// on a dead feed. It may fire more than once (e.g. a stale trip then a channel
// close); the default Shutdown is idempotent and a custom handler must tolerate
// repeats. The action is dispatched on its own goroutine so the events pump can
// return without deadlocking against a handler that joins it via Shutdown.
func (c *Cerebro) onFeedLoss(reason string) {
	c.log.Error("market feed lost", "reason", reason)
	if c.feedLossHandler != nil {
		go c.feedLossHandler(reason)
		return
	}
	go c.Shutdown()
}

// markHeldItemsActive flags each target item the exchange already reports a position
// in as active, so strategies see their pre-existing inventory at start.
func (c *Cerebro) markHeldItemsActive(ctx context.Context, target []*item.Item) {
	positions := c.market.AccountPositions(ctx)
	for i := range positions {
		for j := range target {
			if target[j].Code == positions[i].Item.Code {
				target[j].UpdateStatus(item.Activate)
			}
		}
	}
}

// registerListeners wires every event consumer into the dispatcher synchronously,
// before any producer runs. Register blocks until the listener is live, so an event
// the market emits immediately cannot race ahead of registration and be dropped. The
// broker applies order/balance events (releasing reserved cash, updating balance);
// the monitor enforces exit policies; each strategy engine fans ticks and order
// updates out to its strategies. If ctx is already canceled, Register is a no-op and
// the AfterFunc hook in Start tears everything down.
func (c *Cerebro) registerListeners(ctx context.Context) {
	c.eventEngine.Register(ctx, c.broker)
	if c.monitor != nil {
		c.eventEngine.Register(ctx, c.monitor)
	}
	for i := range c.engines {
		c.eventEngine.Register(ctx, c.engines[i])
	}
}

// spawnStrategies launches each strategy engine's producer goroutines. Listeners are
// already live, so the first ticks they emit are delivered rather than dropped.
func (c *Cerebro) spawnStrategies(ctx context.Context) {
	for i := range c.engines {
		c.wg.Go(func() {
			c.engines[i].Spawn(ctx)
		})
	}
}

// pumpEvents forwards the market's event stream to the dispatcher until the run
// context is canceled or the feed ends. Events are broadcast on eventCtx, which
// outlives this pump (Shutdown joins the pump before stopping the dispatcher), so an
// event the market already emitted when the run context is canceled still reaches the
// listeners instead of being dropped mid-flight.
//
// When live-feed guarding is armed it also runs a staleness watchdog: a feed that
// silently stops (no events, channel never closed) trips onFeedLoss after feedTimeout,
// and a channel close while the run is still live is itself treated as feed loss. The
// watchdog is reset only by data-plane signals (a tick, or a FeedStatusEvent
// heartbeat across a quiet reconnect); sporadic order/balance events do not count as
// the data feed being alive.
func (c *Cerebro) pumpEvents(ctx, eventCtx context.Context) {
	ch := c.market.Events(ctx)

	var watchdog *time.Timer
	if c.feedTimeout > 0 {
		watchdog = time.AfterFunc(c.feedTimeout, func() {
			c.onFeedLoss(fmt.Sprintf("no market data within %s", c.feedTimeout))
		})
		defer watchdog.Stop()
	}

	for {
		select {
		case e, ok := <-ch:
			if !ok {
				// A contract-compliant live adapter reconnects internally and keeps the
				// channel open; closing it while the run is still live means the feed
				// permanently ended, so fail safe when guarding is active. With no guard
				// (e.g. a backtest) a close is the normal end of data.
				if c.feedGuarded() && ctx.Err() == nil {
					c.onFeedLoss("market event channel closed")
				} else {
					c.log.Info("event channel closed")
				}
				return
			}
			if watchdog != nil {
				switch e.(type) {
				case indicator.Tick, market.FeedStatusEvent:
					watchdog.Reset(c.feedTimeout)
				}
			}
			if !c.eventEngine.BroadCastContext(eventCtx, e) {
				return
			}
		case <-ctx.Done():
			c.log.Info("context done")
			c.drainPending(eventCtx, ch)
			return
		}
	}
}

// drainPending forwards events the market already emitted and buffered, without
// blocking, so an in-flight fill still reaches the broker after the run context is
// canceled — then returns rather than blocking on a market that never closes its
// channel.
func (c *Cerebro) drainPending(eventCtx context.Context, ch <-chan any) {
	for {
		select {
		case e, ok := <-ch:
			if !ok || !c.eventEngine.BroadCastContext(eventCtx, e) {
				return
			}
		default:
			return
		}
	}
}

// Shutdown stops cerebro and blocks until everything has drained. Producers are
// torn down in order — spawn/events, then strategy Next goroutines, then broker
// submissions — all while the event dispatcher keeps draining. Only once no
// producer can broadcast anymore is the dispatcher stopped, and it in turn waits
// for its listeners to finish. Shutdown is therefore a barrier: once it returns,
// every dispatched event has been processed, so post-run reads (e.g. Report) are
// complete.
func (c *Cerebro) Shutdown() {
	c.shutdownOnce.Do(c.shutdown)
}

func (c *Cerebro) shutdown() {
	c.log.Info("shutdown")
	if c.cancel != nil {
		c.cancel()
	}

	// 1. Producers on the run context: spawn finishes registering Next goroutines,
	//    the market-events loop exits.
	c.wg.Wait()
	// 2. Long-running strategy Next goroutines.
	for _, e := range c.engines {
		e.Wait()
	}
	// 3. In-flight broker submissions (they broadcast order updates).
	c.broker.Wait()

	// 4. No producer remains, so stop the dispatcher and wait for it. The dispatcher
	//    drains its queues and waits for the per-listener workers, so this join is a
	//    barrier: every event broadcast above has now been processed.
	if c.eventCancel != nil {
		c.eventCancel()
	}
	c.eventWg.Wait()
}
