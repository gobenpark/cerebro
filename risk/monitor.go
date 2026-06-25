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

package risk

import (
	"context"
	"log/slog"
	"sync"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// Submitter is the minimal broker surface the Monitor needs to place exit orders.
// A broker handle scoped to a strategy satisfies it, so exits are attributed back
// to the strategy whose position is being closed.
type Submitter interface {
	Order(ctx context.Context, o order.Order, safe bool) error
}

// Book reports a strategy's current open position in a code (average entry as
// Price), the high-water fill price since it opened, or ok=false if it holds none.
// The broker is the single source of truth for positions, so the Monitor reads
// from it rather than reconstructing its own ledger; the fill high lets a trailing
// stop account for scale-ins above the average entry. A *broker.Broker satisfies it.
type Book interface {
	StrategyPosition(strategy, code string) (pos position.Position, peak decimal.Decimal, ok bool)
}

// Monitor is a reactive risk component. On every tick it reads each policy-bearing
// strategy's position from the Book and checks it against the strategy's Policy,
// submitting a market exit when a stop-loss, trailing-stop, or take-profit trigger
// fires. The broker owns the position ledger; the Monitor only keeps the trailing
// peak and an in-flight exit guard.
//
// It is an event.Listener driven from the dispatcher's single goroutine, so Listen
// is never concurrent with itself. AddPolicy/RemovePolicy, however, are called from
// the screener reconcile goroutine to attach and detach a dynamically spawned
// strategy's exit policy, so mu guards the Monitor's maps against that concurrency.
type Monitor struct {
	logger    *slog.Logger
	submitter func(strategy string) Submitter
	book      Book

	// mu guards policies, peak, and exiting: Listen mutates them on the dispatcher
	// goroutine while AddPolicy/RemovePolicy mutate policies (and clean peak/exiting)
	// from the reconcile goroutine.
	mu       sync.Mutex
	policies map[string]Policy

	// peak[strategy][code] is the highest price seen since entry, driving trailing
	// stops. It folds the broker's high-water fill price with the ticks the Monitor
	// observes (so neither a scale-in above the average nor a dropped tick understates
	// it), and resets when the position goes flat.
	peak map[string]map[string]decimal.Decimal
	// exiting[strategy][code] holds the in-flight exit order id, so a position is
	// not exited again before its first exit settles.
	exiting map[string]map[string]string
}

// NewMonitor builds a Monitor for the given per-strategy policies. submitter maps
// a strategy name to the broker handle its exits are placed through, and book is
// the broker's position ledger the policies are evaluated against.
func NewMonitor(logger *slog.Logger, policies map[string]Policy, submitter func(strategy string) Submitter, book Book) *Monitor {
	return &Monitor{
		logger:    logger,
		policies:  policies,
		submitter: submitter,
		book:      book,
		peak:      map[string]map[string]decimal.Decimal{},
		exiting:   map[string]map[string]string{},
	}
}

// Listen evaluates policies on ticks and releases the exit guard on a terminal
// exit order. It satisfies event.Listener.
func (m *Monitor) Listen(ctx context.Context, e any) {
	switch evt := e.(type) {
	case order.Order:
		m.applyOrder(evt)
	case indicator.Tick:
		m.onTick(ctx, evt)
	}
}

// AddPolicy attaches a dynamically spawned strategy's exit policy under its name, so
// the Monitor enforces stop-loss/trailing/take-profit for screener-spawned items the
// same as for static ones. A disabled policy is ignored.
func (m *Monitor) AddPolicy(name string, p Policy) {
	if !p.Enabled() {
		return
	}
	m.mu.Lock()
	m.policies[name] = p
	m.mu.Unlock()
}

// RemovePolicy detaches a strategy's policy and drops its trailing peak and exit
// guard, so an evicted screener strategy is no longer monitored and a later re-entry
// under the same name starts clean.
func (m *Monitor) RemovePolicy(name string) {
	m.mu.Lock()
	delete(m.policies, name)
	delete(m.peak, name)
	delete(m.exiting, name)
	m.mu.Unlock()
}

// applyOrder reacts to a terminal order for a policy-bearing strategy. If the
// position is now flat — closed by the Monitor's own exit or by the strategy's own
// sell — its trailing peak and guard are dropped so an immediate re-entry (before
// any tick observes the flat state) starts from a fresh high-water mark instead of
// inheriting the prior trade's. The broker updates the position before broadcasting
// the order, so a flat book here reliably means the position closed. If the position
// is still open and this was the Monitor's in-flight exit that did not fill
// (rejected/canceled), only the guard is released so a later breach can retry.
func (m *Monitor) applyOrder(o order.Order) {
	if !isTerminal(o.Status()) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	strategy, code := o.Strategy(), o.Item().Code
	if _, ok := m.policies[strategy]; !ok {
		return // not a policy-bearing strategy
	}
	if _, _, ok := m.book.StrategyPosition(strategy, code); !ok {
		m.resetCode(strategy, code) // flat: drop the guard and the stale trailing peak
		return
	}
	if m.exiting[strategy][code] == o.ID() {
		m.clearExitCode(strategy, code) // our exit failed; allow a retry, keep the peak
	}
}

// onTick evaluates every policy-bearing strategy that holds the tick's code,
// submitting an exit when a trigger fires. A strategy that is flat in the code has
// its trailing peak and any stale guard reset so a re-entry starts clean.
func (m *Monitor) onTick(ctx context.Context, tk indicator.Tick) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for strategy, policy := range m.policies {
		pos, fillPeak, ok := m.book.StrategyPosition(strategy, tk.Code)
		if !ok {
			m.resetCode(strategy, tk.Code)
			continue
		}
		peak := m.updatePeak(strategy, tk.Code, fillPeak, tk.Price)
		if m.hasExit(strategy, tk.Code) {
			continue // an exit is already in flight
		}
		reason, hit := policy.triggered(pos.Price, peak, tk.Price)
		if !hit {
			continue
		}
		m.submitExit(ctx, strategy, pos, tk.Price, reason)
	}
}

// submitExit places a market sell for the whole position and records the in-flight
// exit so the next tick does not double-exit.
func (m *Monitor) submitExit(ctx context.Context, strategy string, pos position.Position, price decimal.Decimal, reason string) {
	if pos.Size.LessThanOrEqual(decimal.Zero) {
		return
	}
	o := order.NewOrder(pos.Item, order.Sell, order.Market, pos.Size, decimal.Zero)
	// safe=false: an exit must go through even if the strategy has other open
	// orders for the code.
	if err := m.submitter(strategy).Order(ctx, o, false); err != nil {
		m.logger.Info("risk policy exit rejected",
			"strategy", strategy, "code", pos.Item.Code, "reason", reason, "error", err)
		return
	}
	m.setExit(strategy, pos.Item.Code, o.ID())
	m.logger.Info("risk policy exit",
		"strategy", strategy, "code", pos.Item.Code, "reason", reason,
		"size", pos.Size, "entry", pos.Price, "price", price)
}

// updatePeak folds the broker's high-water fill price and the latest tick into the
// trailing peak for (strategy, code) and returns it. Combining both sources means
// neither a fill above the average entry (which the broker captures) nor a dropped
// tick (which the Monitor's own stream captures) understates the high-water mark.
func (m *Monitor) updatePeak(strategy, code string, fillPeak, price decimal.Decimal) decimal.Decimal {
	codes := m.peak[strategy]
	if codes == nil {
		codes = map[string]decimal.Decimal{}
		m.peak[strategy] = codes
	}
	p := codes[code] // zero on first observation
	if fillPeak.GreaterThan(p) {
		p = fillPeak
	}
	if price.GreaterThan(p) {
		p = price
	}
	codes[code] = p
	return p
}

// resetCode drops the trailing peak and any stale exit guard for a flat position.
func (m *Monitor) resetCode(strategy, code string) {
	if codes := m.peak[strategy]; codes != nil {
		delete(codes, code)
		if len(codes) == 0 {
			delete(m.peak, strategy)
		}
	}
	m.clearExitCode(strategy, code)
}

func (m *Monitor) setExit(strategy, code, id string) {
	codes := m.exiting[strategy]
	if codes == nil {
		codes = map[string]string{}
		m.exiting[strategy] = codes
	}
	codes[code] = id
}

func (m *Monitor) hasExit(strategy, code string) bool {
	_, ok := m.exiting[strategy][code]
	return ok
}

func (m *Monitor) clearExitCode(strategy, code string) {
	codes := m.exiting[strategy]
	if codes == nil {
		return
	}
	delete(codes, code)
	if len(codes) == 0 {
		delete(m.exiting, strategy)
	}
}

// isTerminal reports whether a status ends an order's lifecycle.
func isTerminal(s order.Status) bool {
	switch s {
	case order.Completed, order.Canceled, order.Expired, order.Margin, order.Rejected:
		return true
	default:
		return false
	}
}
