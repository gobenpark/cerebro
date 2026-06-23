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
	"sync"
	"time"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	log2 "github.com/gobenpark/cerebro/log/v1"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	cancel   context.CancelFunc
	logLevel log.Level
	// broker buy, sell and manage order
	broker *broker.Broker

	target []*item.Item

	market market.Market

	log log.Logger

	// eventEngine engine of management all event
	eventEngine *event.Engine

	strategies []strategy.Strategy

	// risk is the optional pre-trade gate; installed on the broker when set.
	risk *risk.Manager

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
	// eventWg tracks the event dispatcher goroutine.
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
		if c.logLevel == 0 {
			c.logLevel = log.InfoLevel
		}

		logger, err := log2.NewLogger(c.logLevel)
		if err != nil {
			panic(err)
		}
		c.log = logger
	}

	c.broker = broker.NewDefaultBroker(c.eventEngine, c.market, c.log)
	if c.risk != nil {
		c.broker.SetRisk(c.risk)
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
	c.engines = append(c.engines, strategy.NewEngine(c.log, c.broker, c.strategies, c.market, c.timeout))

	return c
}

// Start run cerebro
func (c *Cerebro) Start(ctx context.Context) error {
	// Validate before spawning anything so a bad config leaks no goroutines.
	if len(c.target) == 0 {
		return fmt.Errorf("error need target setting")
	}

	if c.strategies == nil {
		return fmt.Errorf("error empty strategies")
	}

	// A policy must name a registered strategy, else its exits could never trigger
	// (no fills would be attributed to an unknown name). Fail fast on a typo.
	if len(c.policies) > 0 {
		known := make(map[string]struct{}, len(c.strategies))
		for _, st := range c.strategies {
			known[st.Name()] = struct{}{}
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
			c.engines[i].Spawn(ctx, c.target)
		})
	}

	c.wg.Go(func() {
		ch := c.market.Events(ctx)
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					c.log.Info("event channel closed")
					return
				}
				// Stop if the dispatch loop is gone, otherwise this send blocks forever.
				if !c.eventEngine.BroadCastContext(ctx, e) {
					return
				}
			case <-ctx.Done():
				c.log.Info("context done")
				return
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
// producer can broadcast anymore is the dispatcher itself stopped.
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

	// 4. No producer remains, so stop the dispatcher and wait for it.
	if c.eventCancel != nil {
		c.eventCancel()
	}
	c.eventWg.Wait()
}
