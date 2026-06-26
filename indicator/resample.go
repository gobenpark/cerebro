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

package indicator

import (
	"slices"
	"time"
)

// Resampling groups ticks into fixed-width OHLCV candles. A tick belongs to the
// bucket [start, start+compress) where start = tick.Date truncated to compress.
// The incremental Resampler and the batch Resample share the same bucket boundary
// and fold logic below, so they always agree on which bucket a tick lands in.

// newBucket seeds a candle from the tick that opens its bucket.
func newBucket(tk Tick, compress time.Duration) *Candle {
	return &Candle{
		Date:   tk.Date.Truncate(compress),
		Code:   tk.Code,
		Open:   tk.Price,
		High:   tk.Price,
		Low:    tk.Price,
		Close:  tk.Price,
		Volume: tk.Volume,
	}
}

// fold merges a tick into its open bucket: the close advances, the high/low
// extend, and volume accumulates. The open is fixed by the bucket's first tick.
func fold(c *Candle, tk Tick) {
	c.Close = tk.Price
	if tk.Price.GreaterThan(c.High) {
		c.High = tk.Price
	}
	if tk.Price.LessThan(c.Low) {
		c.Low = tk.Price
	}
	c.Volume += tk.Volume
}

// inBucket reports whether tk falls in c's bucket, i.e. before the next bucket
// boundary. A tick exactly on the boundary (c.Date+compress) opens a new bucket.
func inBucket(c *Candle, tk Tick, compress time.Duration) bool {
	return tk.Date.Before(c.Date.Add(compress))
}

// Resampler incrementally folds a single instrument's ticks into fixed-width
// OHLCV candles. It is built for a strategy's tick loop: feed each tick to Add,
// which returns the bar that just closed whenever a tick opens a new bucket, so
// signals can be computed on completed bars without diffing slices.
//
// A Resampler is not safe for concurrent use; a strategy drives it from its own
// goroutine.
type Resampler struct {
	compress time.Duration
	window   int     // max closed bars retained; 0 means unbounded
	open     *Candle // the bar currently forming; nil before the first tick
	history  Candles // closed bars, oldest first
}

// ResamplerOption configures a Resampler.
type ResamplerOption func(*Resampler)

// WithWindow caps the number of closed bars History retains, dropping the oldest
// so a long-running strategy does not accumulate candles without bound. A value
// of n <= 0 keeps an unbounded history (the default).
func WithWindow(n int) ResamplerOption {
	return func(r *Resampler) { r.window = n }
}

// WithSeed pre-loads the Resampler's closed-bar history with past candles, so
// History (and the indicators computed from it) are warm from the first live tick
// instead of starting empty. It clones history, leaving the caller's slice
// untouched; the seed is bounded to WithWindow like any other closed bar. The
// seeded bars should be older than the first tick fed to Add (they are treated as
// already closed and are never re-folded).
func WithSeed(history Candles) ResamplerOption {
	return func(r *Resampler) { r.history = slices.Clone(history) }
}

// NewResampler returns a Resampler that buckets ticks into compress-wide candles.
func NewResampler(compress time.Duration, opts ...ResamplerOption) *Resampler {
	r := &Resampler{compress: compress}
	for _, o := range opts {
		o(r)
	}
	r.trim() // bound a seeded history to the window regardless of option order
	return r
}

// Add folds tk into the forming bar. When tk falls into a new bucket the forming
// bar is completed — appended to History and returned with ok=true — before the
// new bar opens; otherwise it returns ok=false. Ticks are assumed to arrive in
// non-decreasing time order.
func (r *Resampler) Add(tk Tick) (closed Candle, ok bool) {
	if r.open == nil {
		r.open = newBucket(tk, r.compress)
		return Candle{}, false
	}
	if inBucket(r.open, tk, r.compress) {
		fold(r.open, tk)
		return Candle{}, false
	}

	done := r.open
	r.history = append(r.history, done)
	r.trim()
	r.open = newBucket(tk, r.compress)
	return *done, true
}

// Current returns a snapshot of the bar currently forming (not yet closed), or
// ok=false before the first tick.
func (r *Resampler) Current() (Candle, bool) {
	if r.open == nil {
		return Candle{}, false
	}
	return *r.open, true
}

// History returns the closed bars, oldest first, within the configured window.
// The slice is owned by the Resampler and is valid until the next Add; copy it if
// it must outlive that.
func (r *Resampler) History() Candles {
	return r.history
}

// trim bounds History to the window, shifting the most recent bars to the front
// and clearing the freed tail so dropped candles can be collected. It runs only
// when a bar closes (at most once per compress interval), so the copy is cheap.
func (r *Resampler) trim() {
	if r.window <= 0 || len(r.history) <= r.window {
		return
	}
	n := copy(r.history, r.history[len(r.history)-r.window:])
	clear(r.history[n:])
	r.history = r.history[:n]
}

// Resample groups a batch of ticks into candles. It sorts a copy by time, so the
// caller's slice is left untouched. Use it for backtests or precomputed history;
// use a Resampler for a live tick loop.
func Resample(tk []Tick, compress time.Duration) Candles {
	sorted := slices.Clone(tk)
	slices.SortFunc(sorted, func(a, b Tick) int {
		return a.Date.Compare(b.Date)
	})

	cds := Candles{}
	for i := range sorted {
		if len(cds) == 0 {
			cds = append(cds, newBucket(sorted[i], compress))
			continue
		}
		last := cds[len(cds)-1]
		if inBucket(last, sorted[i], compress) {
			fold(last, sorted[i])
		} else {
			cds = append(cds, newBucket(sorted[i], compress))
		}
	}
	return cds
}
