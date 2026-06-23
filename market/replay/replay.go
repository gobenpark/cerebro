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

// Package replay provides a market.Market implementation that replays historical
// candles and simulates order fills locally. It needs no real exchange, so it is
// the reference adapter for backtests, demos, and end-to-end tests.
//
// Determinism note: Cerebro's live pipeline fans ticks out non-blocking (a slow
// strategy drops ticks) and submits orders asynchronously, so replay is paced by
// Interval rather than run in strict lock-step. It is faithful enough for demos
// and integration tests; a bit-for-bit deterministic backtest engine would need
// synchronous stepping and is a separate concern.
package replay

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// Replay is a simulated market: it streams historical candles as ticks and fills
// submitted orders against those prices while tracking cash and positions locally.
type Replay struct {
	mu         sync.Mutex
	balance    decimal.Decimal
	commission decimal.Decimal // percentage, e.g. 0.015 == 0.015%
	interval   time.Duration   // pacing between emitted ticks
	data       map[string]indicator.Candles
	positions  map[string]position.Position
	pending    []order.Order
	// subscribed marks codes a strategy has subscribed to; ready[code] is closed
	// when that code is subscribed, releasing its per-code emitter. A code is
	// replayed only after it is subscribed, so a loaded-but-untargeted code neither
	// emits nor blocks the others — no data loss, no hang.
	subscribed map[string]struct{}
	ready      map[string]chan struct{}
	// done is closed when the replay loop ends (every code's candles exhausted, or
	// the context is canceled), giving callers a clean "backtest finished" signal.
	done chan struct{}
}

// Ensure Replay satisfies the market interface at compile time.
var _ market.Market = (*Replay)(nil)

type Option func(*Replay)

// WithBalance seeds the simulated settled cash.
func WithBalance(b decimal.Decimal) Option { return func(r *Replay) { r.balance = b } }

// WithCommission sets the percentage fee charged on each fill.
func WithCommission(c decimal.Decimal) Option { return func(r *Replay) { r.commission = c } }

// WithInterval paces the gap between emitted ticks. A small non-zero value keeps
// the async pipeline able to react (place orders) before the next tick.
func WithInterval(d time.Duration) Option { return func(r *Replay) { r.interval = d } }

// WithCandles registers the historical candles to replay for a code. It may be
// called multiple times to load several instruments.
func WithCandles(code string, candles indicator.Candles) Option {
	return func(r *Replay) { r.data[code] = candles }
}

func New(opts ...Option) *Replay {
	r := &Replay{
		data:       map[string]indicator.Candles{},
		positions:  map[string]position.Position{},
		subscribed: map[string]struct{}{},
		done:       make(chan struct{}),
	}
	for _, o := range opts {
		o(r)
	}
	// One readiness gate per loaded code, closed when that code is subscribed.
	r.ready = make(map[string]chan struct{}, len(r.data))
	for code := range r.data {
		r.ready[code] = make(chan struct{})
	}
	return r
}

func (r *Replay) Stocks(_ context.Context) []*item.Item {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*item.Item, 0, len(r.data))
	for code := range r.data {
		out = append(out, &item.Item{Code: code})
	}
	return out
}

func (r *Replay) Candles(_ context.Context, code string, _ market.CandleType) (indicator.Candles, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data[code], nil
}

// Subscribe records the handler's codes and releases each one's emitter. A code
// is replayed only after it is subscribed, so its consumer channel exists before
// the first tick and untargeted codes are simply never emitted.
func (r *Replay) Subscribe(handler market.TickEventHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range handler() {
		if _, done := r.subscribed[it.Code]; done {
			continue
		}
		r.subscribed[it.Code] = struct{}{}
		if c, ok := r.ready[it.Code]; ok {
			close(c) // release this code's emitter
		}
	}
	return nil
}

// Order records the order; the replay loop fills it when a tick crosses its price.
func (r *Replay) Order(_ context.Context, o order.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = append(r.pending, o)
	return nil
}

func (r *Replay) AccountPositions() []position.Position {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]position.Position, 0, len(r.positions))
	for _, p := range r.positions {
		if p.Size.GreaterThan(decimal.Zero) {
			out = append(out, p)
		}
	}
	return out
}

func (r *Replay) AccountBalance() decimal.Decimal {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.balance
}

// Commission is immutable after New, so it needs no lock.
func (r *Replay) Commission() decimal.Decimal { return r.commission }

// Events returns the simulated event stream and starts one emitter per loaded
// code. The channel is closed once every emitter has finished (each targeted
// code's candles are exhausted; untargeted codes wait until ctx is canceled).
func (r *Replay) Events(ctx context.Context) <-chan any {
	ch := make(chan any, 16)
	go r.run(ctx, ch)
	return ch
}

// Done is closed once the replay loop has finished (all subscribed codes' candles
// have been replayed, or the context was canceled). Useful to detect backtest end.
func (r *Replay) Done() <-chan struct{} { return r.done }

func (r *Replay) run(ctx context.Context, ch chan any) {
	defer close(r.done)
	defer close(ch)

	r.mu.Lock()
	codes := make([]string, 0, len(r.data))
	for code := range r.data {
		codes = append(codes, code)
	}
	r.mu.Unlock()

	var wg sync.WaitGroup
	for _, code := range codes {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			r.emit(ctx, ch, code)
		}(code)
	}
	wg.Wait()
}

// emit waits for code to be subscribed, then replays its candles in time order,
// filling pending orders against each price. It returns on context cancellation,
// so a code that is never subscribed just idles until shutdown.
func (r *Replay) emit(ctx context.Context, ch chan any, code string) {
	r.mu.Lock()
	ready := r.ready[code]
	r.mu.Unlock()

	select {
	case <-ready:
	case <-ctx.Done():
		return
	}

	for _, c := range r.barsFor(code) {
		tk := indicator.Tick{Code: code, Date: c.Date, Price: c.Close, Volume: c.Volume}
		if !send(ctx, ch, tk) {
			return
		}
		for _, e := range r.matchAndFill(code, c.Close) {
			if !send(ctx, ch, e) {
				return
			}
		}
		if r.interval > 0 {
			select {
			case <-time.After(r.interval):
			case <-ctx.Done():
				return
			}
		}
	}
}

// barsFor returns code's candles in ascending time order.
func (r *Replay) barsFor(code string) []*indicator.Candle {
	r.mu.Lock()
	defer r.mu.Unlock()
	cs := r.data[code]
	out := make([]*indicator.Candle, len(cs))
	copy(out, cs)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date.Before(out[j].Date)
	})
	return out
}

// matchAndFill fills every pending order for code whose price condition is met at
// the given price, updates cash/positions, and returns the resulting events.
func (r *Replay) matchAndFill(code string, price decimal.Decimal) []any {
	r.mu.Lock()
	defer r.mu.Unlock()

	var events []any
	still := r.pending[:0:0]
	for _, o := range r.pending {
		fillPrice, ok := fillPriceFor(o, code, price)
		if !ok {
			still = append(still, o)
			continue
		}
		// No shorting: a sell only fills if the held position can cover it,
		// otherwise it would mint cash and drive the position negative.
		if o.Action() == order.Sell && r.positions[code].Size.LessThan(o.Size()) {
			still = append(still, o)
			continue
		}
		r.execute(o, fillPrice)
		o.Complete()
		events = append(events,
			market.ChangeBalanceEvent{Message: "fill", Balance: r.balance},
			market.ChangeOrderEvent{
				ID: o.ID(), Action: order.Completed, Message: "filled",
				FilledSize: o.Size(), Price: fillPrice,
			},
		)
	}
	r.pending = still
	return events
}

// fillPriceFor reports the fill price for o at the current price, or ok=false if
// the order should stay open. Market orders fill at the current price; limit buys
// fill at their limit once price drops to it, limit sells once price rises to it.
func fillPriceFor(o order.Order, code string, price decimal.Decimal) (decimal.Decimal, bool) {
	if o.Item().Code != code {
		return decimal.Zero, false
	}
	if o.Type() == order.Limit {
		switch o.Action() {
		case order.Buy:
			if price.LessThanOrEqual(o.Price()) {
				return o.Price(), true
			}
		case order.Sell:
			if price.GreaterThanOrEqual(o.Price()) {
				return o.Price(), true
			}
		default:
			// Cancel/Edit are not fillable; leave the order open.
		}
		return decimal.Zero, false
	}
	// Market (and any non-limit) orders fill at the current price.
	return price, true
}

// execute applies a fill to cash and the position book. Caller holds r.mu.
func (r *Replay) execute(o order.Order, price decimal.Decimal) {
	notional := price.Mul(o.Size())
	fee := notional.Mul(r.commission).Div(decimal.NewFromInt(100))
	code := o.Item().Code
	p := r.positions[code]
	p.Item = o.Item()

	switch o.Action() {
	case order.Buy:
		r.balance = r.balance.Sub(notional).Sub(fee)
		newSize := p.Size.Add(o.Size())
		if newSize.GreaterThan(decimal.Zero) {
			cost := p.Size.Mul(p.Price).Add(notional)
			p.Price = cost.Div(newSize)
		}
		p.Size = newSize
	case order.Sell:
		r.balance = r.balance.Add(notional).Sub(fee)
		p.Size = p.Size.Sub(o.Size())
	default:
		// Cancel/Edit do not move cash or positions.
	}
	r.positions[code] = p
}

func send(ctx context.Context, ch chan any, v any) bool {
	select {
	case ch <- v:
		return true
	case <-ctx.Done():
		return false
	}
}
