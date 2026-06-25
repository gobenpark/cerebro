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
	"log/slog"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

// liveCode is the per-item strategy the reconciler currently runs for one code: the
// item (for the eviction context) and the strategy's name (to evict it).
type liveCode struct {
	item *item.Item
	name string
}

// reconciler converges one screening group's per-item strategies to its screener's
// watchlist snapshots: it spawns a strategy from the group's factory for each newly
// screened code, and per the group's EvictionPolicy retires those whose code dropped
// out. It runs on a single goroutine (the group's screener loop), so its map needs no
// locking. Each group has its own reconciler, so several screeners drive several
// strategies independently.
type reconciler struct {
	log     *slog.Logger
	engine  *strategy.Engine
	broker  *broker.Broker
	monitor *risk.Monitor // may be nil when no exit policies are in play
	// overrides are the explicit WithRiskPolicy entries, keyed by strategy name. An
	// enabled one replaces a spawned strategy's declared ExitPolicy; a disabled one
	// clears it — the same precedence buildMonitor applies to static runners.
	overrides map[string]risk.Policy
	factory   func(*item.Item) strategy.Strategy
	evict     EvictionPolicy
	live      map[string]*liveCode
}

func newReconciler(log *slog.Logger, eng *strategy.Engine, bk *broker.Broker, mon *risk.Monitor, overrides map[string]risk.Policy, factory func(*item.Item) strategy.Strategy, evict EvictionPolicy) *reconciler {
	return &reconciler{
		log:       log,
		engine:    eng,
		broker:    bk,
		monitor:   mon,
		overrides: overrides,
		factory:   factory,
		evict:     evict,
		live:      map[string]*liveCode{},
	}
}

// exitPolicy resolves the effective reactive exit policy for a spawned strategy: an
// explicit WithRiskPolicy override (enabled wins, disabled clears) takes precedence
// over the strategy's own declared ExitPolicy (strategy.RiskAware). ok is false when
// no enabled policy applies.
func (r *reconciler) exitPolicy(st strategy.Strategy) (risk.Policy, bool) {
	if p, ok := r.overrides[st.Name()]; ok {
		return p, p.Enabled()
	}
	if ra, ok := st.(strategy.RiskAware); ok {
		p := ra.ExitPolicy()
		return p, p.Enabled()
	}
	return risk.Policy{}, false
}

// run drives reconcile from each watchlist snapshot until ctx is canceled or the
// screener's channel closes (a static screen, or a feed that ended).
func (r *reconciler) run(ctx context.Context, ch <-chan []*item.Item) {
	for {
		select {
		case <-ctx.Done():
			return
		case desired, ok := <-ch:
			if !ok {
				return
			}
			r.reconcile(ctx, desired)
		}
	}
}

// hasOpenOrder reports whether the named strategy still has an open order for code —
// an unfilled entry or an in-flight exit — so eviction can treat it as not-yet-settled
// even when its position reads flat.
func (r *reconciler) hasOpenOrder(name, code string) bool {
	for _, o := range r.broker.Orders(code) {
		if o.Strategy() == name {
			return true
		}
	}
	return false
}

// reconcile spawns a strategy for newly screened codes and evicts (per policy) those
// no longer screened. desired is the full snapshot, so membership is declarative.
func (r *reconciler) reconcile(ctx context.Context, desired []*item.Item) {
	want := make(map[string]*item.Item, len(desired))
	for _, it := range desired {
		want[it.Code] = it
	}

	// Spawn a strategy for codes that newly qualified.
	for code, it := range want {
		if _, ok := r.live[code]; ok {
			continue // already running
		}
		st := r.factory(it)
		if err := r.engine.AddRunner(ctx, strategy.Runner{Strategy: st, Items: []*item.Item{it}}); err != nil {
			r.log.Error("screen: add runner", "code", code, "strategy", st.Name(), "err", err)
			continue
		}
		// Attach the strategy's effective exit policy (explicit override or its own
		// declared RiskAware policy) to the monitor, so screener-spawned items get
		// stop-loss/take-profit just like static ones.
		if r.monitor != nil {
			if p, ok := r.exitPolicy(st); ok {
				r.monitor.AddPolicy(st.Name(), p)
			}
		}
		r.live[code] = &liveCode{item: it, name: st.Name()}
	}

	// Retire strategies for codes that dropped out, subject to the eviction policy.
	for code, lc := range r.live {
		if _, ok := want[code]; ok {
			continue
		}
		pos, _, _ := r.broker.StrategyPosition(lc.name, code)
		e := Eviction{Strategy: lc.name, Item: lc.item, Position: pos, Pending: r.hasOpenOrder(lc.name, code)}
		if !r.evict(ctx, e, r.broker.Scoped(lc.name)) {
			continue // policy kept it (e.g. still holding a position); re-check next snapshot
		}
		r.engine.RemoveRunner(ctx, lc.name)
		if r.monitor != nil {
			r.monitor.RemovePolicy(lc.name)
		}
		delete(r.live, code)
	}
}
