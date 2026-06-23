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

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/order"
)

// Submitter is the minimal broker surface the Monitor needs to place exit orders.
// A broker handle scoped to a strategy satisfies it, so exits are attributed back
// to the strategy whose position is being closed.
type Submitter interface {
	Order(ctx context.Context, o order.Order, safe bool) error
}

// lot is the Monitor's reconstruction of one strategy's long position in one code,
// built from that strategy's attributed fills.
type lot struct {
	item  *item.Item
	size  decimal.Decimal // net long size
	price decimal.Decimal // average entry price
	peak  decimal.Decimal // highest price seen since entry (drives trailing stops)
}

// Monitor is a reactive risk component. It tracks each policy-bearing strategy's
// position from the attributed fills it observes, and on every tick checks that
// position against the strategy's Policy, submitting a market exit when a trigger
// fires.
//
// It is an event.Listener. The event engine drives each listener from a single
// dedicated goroutine, so Listen is never called concurrently with itself; the
// Monitor's state therefore needs no locking.
type Monitor struct {
	logger    log.Logger
	policies  map[string]Policy
	submitter func(strategy string) Submitter

	// lots[strategy][code] is the position each policy is evaluated against.
	lots map[string]map[string]*lot
	// filled[orderID] is the cumulative size already counted for an order, so a
	// partial fill followed by completion is applied exactly once.
	filled map[string]decimal.Decimal
	// exiting[strategy][code] holds the in-flight exit order id, so a position is
	// not exited again before its first exit settles.
	exiting map[string]map[string]string
	// last[code] is the most recent tick price, used as the fill price for market
	// (zero-price) orders, whose actual fill price the event does not carry.
	last map[string]decimal.Decimal
}

// NewMonitor builds a Monitor for the given per-strategy policies. submitter maps
// a strategy name to the broker handle its exits are placed through.
func NewMonitor(logger log.Logger, policies map[string]Policy, submitter func(strategy string) Submitter) *Monitor {
	return &Monitor{
		logger:    logger,
		policies:  policies,
		submitter: submitter,
		lots:      map[string]map[string]*lot{},
		filled:    map[string]decimal.Decimal{},
		exiting:   map[string]map[string]string{},
		last:      map[string]decimal.Decimal{},
	}
}

// Listen consumes order updates (to track positions) and ticks (to evaluate the
// policies). It satisfies event.Listener.
func (m *Monitor) Listen(ctx context.Context, e any) {
	switch evt := e.(type) {
	case order.Order:
		m.applyOrder(evt)
	case indicator.Tick:
		m.onTick(ctx, evt)
	}
}

// applyOrder folds an order update into the per-strategy position. Only fills move
// the position; a terminal exit order releases the in-flight guard.
func (m *Monitor) applyOrder(o order.Order) {
	strategy := o.Strategy()
	if _, ok := m.policies[strategy]; !ok {
		return // not a policy-bearing strategy
	}
	code := o.Item().Code
	status := o.Status()

	// Count the size newly filled by this update. A partial fill shrinks the
	// remaining size; completion zeroes it. Tracking cumulative filled size per
	// order id makes a partial-then-complete sequence add up to the full size once.
	if status == order.Partial || status == order.Completed {
		cumulative := o.Size().Sub(o.RemainingSize())
		delta := cumulative.Sub(m.filled[o.ID()])
		if status == order.Completed {
			delete(m.filled, o.ID())
		} else {
			m.filled[o.ID()] = cumulative
		}
		if delta.GreaterThan(decimal.Zero) {
			price := o.Price()
			if price.IsZero() { // market fill: approximate with the last tick price
				price = m.last[code]
			}
			switch o.Action() {
			case order.Buy:
				m.addLong(strategy, o.Item(), delta, price)
			case order.Sell:
				m.reduceLong(strategy, code, delta)
			default:
				// Cancel/Edit are not fills and do not move the position.
			}
		}
	}

	// A terminal exit order frees the guard so a later breach can exit a re-entered
	// position. Completion is also handled by reduceLong; clearing twice is safe.
	if isTerminal(status) {
		m.clearExit(strategy, code, o.ID())
	}
}

// addLong adds size at price to a strategy's lot, weighting the average entry.
func (m *Monitor) addLong(strategy string, it *item.Item, size, price decimal.Decimal) {
	codes := m.lots[strategy]
	if codes == nil {
		codes = map[string]*lot{}
		m.lots[strategy] = codes
	}
	l := codes[it.Code]
	if l == nil {
		l = &lot{item: it, peak: price}
		codes[it.Code] = l
	}
	newSize := l.size.Add(size)
	if newSize.GreaterThan(decimal.Zero) {
		cost := l.size.Mul(l.price).Add(size.Mul(price))
		l.price = cost.Div(newSize)
	}
	l.size = newSize
	if price.GreaterThan(l.peak) {
		l.peak = price
	}
}

// reduceLong shrinks a strategy's lot; a flat position is dropped along with its
// peak and exit guard so a fresh entry starts clean.
func (m *Monitor) reduceLong(strategy, code string, size decimal.Decimal) {
	codes := m.lots[strategy]
	l := codes[code]
	if l == nil {
		return
	}
	l.size = l.size.Sub(size)
	if l.size.LessThanOrEqual(decimal.Zero) {
		delete(codes, code)
		if len(codes) == 0 {
			delete(m.lots, strategy)
		}
		m.clearExitCode(strategy, code)
	}
}

// onTick refreshes the trailing peak and evaluates every policy-bearing strategy
// that holds the tick's code, submitting an exit when a trigger fires.
func (m *Monitor) onTick(ctx context.Context, tk indicator.Tick) {
	m.last[tk.Code] = tk.Price

	for strategy, codes := range m.lots {
		l := codes[tk.Code]
		if l == nil {
			continue
		}
		if tk.Price.GreaterThan(l.peak) {
			l.peak = tk.Price
		}
		if m.hasExit(strategy, tk.Code) {
			continue // an exit is already in flight
		}
		reason, ok := m.policies[strategy].triggered(l.price, l.peak, tk.Price)
		if !ok {
			continue
		}
		m.submitExit(ctx, strategy, l, reason)
	}
}

// submitExit places a market sell for the whole position and records the in-flight
// exit so the next tick does not double-exit.
func (m *Monitor) submitExit(ctx context.Context, strategy string, l *lot, reason string) {
	if l.size.LessThanOrEqual(decimal.Zero) {
		return
	}
	o := order.NewOrder(l.item, order.Sell, order.Market, l.size, decimal.Zero)
	// safe=false: an exit must go through even if the strategy has other open
	// orders for the code.
	if err := m.submitter(strategy).Order(ctx, o, false); err != nil {
		m.logger.Info("risk policy exit rejected",
			"strategy", strategy, "code", l.item.Code, "reason", reason, "error", err)
		return
	}
	m.setExit(strategy, l.item.Code, o.ID())
	m.logger.Info("risk policy exit",
		"strategy", strategy, "code", l.item.Code, "reason", reason,
		"size", l.size, "entry", l.price, "price", m.last[l.item.Code])
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

// clearExit releases the guard only if id matches the in-flight exit, so an
// unrelated order's terminal update does not clear a live exit.
func (m *Monitor) clearExit(strategy, code, id string) {
	if m.exiting[strategy][code] == id {
		m.clearExitCode(strategy, code)
	}
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
