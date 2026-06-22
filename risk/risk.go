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

// Package risk provides a pre-trade risk gate: a set of composable rules the
// broker consults before submitting an order. Rules are configured once at
// startup (cerebro.WithRisk) and may be built-in or custom.
package risk

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

var (
	ErrPositionCap = errors.New("order exceeds max position size")
	ErrOrderTooBig = errors.New("order value exceeds limit")
	ErrTooManyOpen = errors.New("too many open positions")
	ErrRateLimited = errors.New("order rate limit exceeded")
)

// Snapshot is the account state a Rule sees when vetting an order.
type Snapshot struct {
	Balance   decimal.Decimal // settled cash
	Available decimal.Decimal // settled cash minus cash reserved by open orders
	Positions []position.Position
	Open      []order.Order
}

// Rule vets a prospective order; a non-nil error rejects it.
type Rule interface {
	Name() string
	Check(o order.Order, s Snapshot) error
}

// Manager runs the configured rules in order.
type Manager struct {
	rules []Rule
}

func New(rules ...Rule) *Manager { return &Manager{rules: rules} }

// Check rejects the order if any rule fails.
func (m *Manager) Check(o order.Order, s Snapshot) error {
	for _, r := range m.rules {
		if err := r.Check(o, s); err != nil {
			return fmt.Errorf("risk %s: %w", r.Name(), err)
		}
	}
	return nil
}

// ruleFunc adapts a function to Rule.
type ruleFunc struct {
	name string
	fn   func(order.Order, Snapshot) error
}

func (r ruleFunc) Name() string                          { return r.name }
func (r ruleFunc) Check(o order.Order, s Snapshot) error { return r.fn(o, s) }

// Func wraps an inline check as a custom Rule.
func Func(name string, fn func(order.Order, Snapshot) error) Rule {
	return ruleFunc{name: name, fn: fn}
}

// MaxPositionPct rejects a buy that would push one code's exposure (its existing
// position at average cost plus this order) above maxPct of settled balance.
// Market orders (price 0) carry no notional and are not constrained by this rule.
func MaxPositionPct(maxPct float64) Rule {
	limit := decimal.NewFromFloat(maxPct)
	return ruleFunc{"max_position_pct", func(o order.Order, s Snapshot) error {
		if o.Action() != order.Buy {
			return nil
		}
		maxExposure := s.Balance.Mul(limit)
		exposure := o.OrderPrice()
		for _, p := range s.Positions {
			if p.Item != nil && p.Item.Code == o.Item().Code {
				exposure = exposure.Add(p.Price.Mul(p.Size))
			}
		}
		// Pending buys for the same code also commit exposure before they settle;
		// count their unfilled remainder so a burst of orders can't slip past.
		for _, op := range s.Open {
			if op.Action() == order.Buy && op.Item().Code == o.Item().Code {
				exposure = exposure.Add(op.RemainPrice())
			}
		}
		if exposure.GreaterThan(maxExposure) {
			return ErrPositionCap
		}
		return nil
	}}
}

// MaxOrderValue rejects an order whose notional (price*size) exceeds maxValue.
func MaxOrderValue(maxValue decimal.Decimal) Rule {
	return ruleFunc{"max_order_value", func(o order.Order, _ Snapshot) error {
		if o.OrderPrice().GreaterThan(maxValue) {
			return ErrOrderTooBig
		}
		return nil
	}}
}

// MaxOpenPositions rejects a buy in a new code once n distinct positions are held.
// Adding to an already-held position is always allowed.
func MaxOpenPositions(n int) Rule {
	return ruleFunc{"max_open_positions", func(o order.Order, s Snapshot) error {
		if o.Action() != order.Buy {
			return nil
		}
		// Distinct codes already committed: held positions plus pending buys, so
		// several new-code buys submitted before any fills still count.
		codes := make(map[string]struct{})
		for _, p := range s.Positions {
			if p.Size.GreaterThan(decimal.Zero) && p.Item != nil {
				codes[p.Item.Code] = struct{}{}
			}
		}
		for _, op := range s.Open {
			if op.Action() == order.Buy {
				codes[op.Item().Code] = struct{}{}
			}
		}
		if _, ok := codes[o.Item().Code]; ok {
			return nil // adding to an already-held or pending code is fine
		}
		if len(codes) >= n {
			return ErrTooManyOpen
		}
		return nil
	}}
}

// OrderRateLimit rejects an order once n orders have been seen within per. It is
// attempt-based: every checked order counts toward the sliding window.
func OrderRateLimit(n int, per time.Duration) Rule {
	return &rateLimiter{n: n, per: per}
}

type rateLimiter struct {
	mu     sync.Mutex
	n      int
	per    time.Duration
	stamps []time.Time
}

func (r *rateLimiter) Name() string { return "order_rate_limit" }

func (r *rateLimiter) Check(_ order.Order, _ Snapshot) error {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.per)
	r.stamps = slices.DeleteFunc(r.stamps, func(t time.Time) bool { return t.Before(cutoff) })
	if len(r.stamps) >= r.n {
		return ErrRateLimited
	}
	r.stamps = append(r.stamps, now)
	return nil
}
