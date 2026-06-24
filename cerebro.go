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
	"sync"
	"time"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
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
func (c *Cerebro) resolveRunners() ([]strategy.Runner, error) {
	byCode := make(map[string]*item.Item, len(c.target))
	for _, it := range c.target {
		byCode[it.Code] = it
	}

	var runners []strategy.Runner
	for _, reg := range c.stratRegs {
		items := c.target
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
		for _, it := range c.target {
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

// Start run cerebro
func (c *Cerebro) Start(ctx context.Context) error {
	// Validate before spawning anything so a bad config leaks no goroutines.
	if len(c.target) == 0 {
		return fmt.Errorf("error need target setting")
	}

	if len(c.stratRegs) == 0 && len(c.forEachRegs) == 0 {
		return fmt.Errorf("error empty strategies")
	}

	// Resolve registrations into the runners the engine executes. This validates
	// referenced codes and name uniqueness, so a bad config fails before anything spawns.
	runners, err := c.resolveRunners()
	if err != nil {
		return err
	}

	// A policy must name a running strategy, else its exits could never trigger
	// (no fills would be attributed to an unknown name). Fail fast on a typo.
	if len(c.policies) > 0 {
		known := make(map[string]struct{}, len(runners))
		for _, r := range runners {
			known[r.Strategy.Name()] = struct{}{}
		}
		for name, p := range c.policies {
			if !p.Enabled() {
				continue // a disabled (empty) policy is dropped as inert; don't reject it
			}
			if _, ok := known[name]; !ok {
				return fmt.Errorf("risk policy references unknown strategy %q", name)
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// The dispatcher runs on its own context so it can keep draining broadcasts
	// until every producer (spawn, events, strategies, broker) has stopped. It is
	// detached from parent cancellation and stopped only by Shutdown.
	eventCtx, eventCancel := context.WithCancel(context.WithoutCancel(ctx))
	c.eventCancel = eventCancel

	c.log.Info("Cerebro starting ...")

	// Restore any persisted ledger before listeners go live, so the broker's
	// per-strategy PnL/fees/open lots are in place before the first fill event.
	if err := c.broker.Restore(ctx); err != nil {
		return fmt.Errorf("restore broker ledger: %w", err)
	}

	// Build the strategy engine only after every fallible step has succeeded, and
	// assign (not append) so retrying Start after an earlier failure does not leave
	// a stale engine behind that would double-register and double-spawn.
	c.engines = []engine.Engine{strategy.NewEngine(c.log, c.broker, runners, c.market, c.timeout)}

	positions := c.market.AccountPositions()
	for i := range positions {
		for j := range c.target {
			if c.target[j].Code == positions[i].Item.Code {
				c.target[j].UpdateStatus(item.Activate)
			}
		}
	}

	c.eventWg.Go(func() {
		c.eventEngine.Start(eventCtx)
	})

	// Register every listener synchronously BEFORE the market-events pump starts.
	// Register blocks until the dispatcher has the listener live, so an event the
	// market emits immediately cannot race ahead of registration and be dropped.
	// The broker applies order/balance events (releasing reserved cash, updating
	// balance); each strategy engine fans ticks and order updates out to its
	// strategies. If ctx is already canceled, Register is a no-op and the AfterFunc
	// hook below tears everything down.
	c.eventEngine.Register(ctx, c.broker)
	if c.monitor != nil {
		// The monitor watches attributed fills and ticks to enforce exit policies.
		c.eventEngine.Register(ctx, c.monitor)
	}
	for i := range c.engines {
		c.eventEngine.Register(ctx, c.engines[i])
	}

	// Listeners are live; now spawn the strategy producers.
	for i := range c.engines {
		c.wg.Go(func() {
			c.engines[i].Spawn(ctx)
		})
	}

	c.wg.Go(func() {
		ch := c.market.Events(ctx)
		// Forward on the dispatcher's context (eventCtx), which outlives this pump
		// (Shutdown joins the pump before stopping the dispatcher), so an event the
		// market already emitted when the run context is canceled still reaches the
		// listeners instead of being dropped mid-flight.
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					c.log.Info("event channel closed")
					return
				}
				if !c.eventEngine.BroadCastContext(eventCtx, e) {
					return
				}
			case <-ctx.Done():
				c.log.Info("context done")
				// Best-effort: forward events the market already emitted and buffered so
				// an in-flight fill still reaches the broker, then stop without blocking
				// on a market that never closes its channel.
				for {
					select {
					case e, ok := <-ch:
						if !ok {
							return
						}
						if !c.eventEngine.BroadCastContext(eventCtx, e) {
							return
						}
					default:
						return
					}
				}
			}
		}
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
