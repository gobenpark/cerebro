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
	"slices"
	"sync/atomic"
	"time"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/market"
)

// CandleStream is a warm, live candle feed for one (code, level): seeded with the
// adapter's historical candles so indicators are valid immediately, then advanced
// as the universe's live ticks close new bars.
type CandleStream interface {
	// History returns the warm series (seeded history plus the bars that have closed
	// live), oldest first, as a snapshot safe to read from the strategy goroutine. It
	// is the complete record — feed it to the indicator methods on indicator.Candles.
	History() indicator.Candles
	// Closed signals each bar as it closes, carrying that bar. It is a best-effort
	// wakeup: a strategy that falls behind drops signals (History stays complete), so
	// recompute from History rather than treating Closed as a lossless log. The
	// channel is closed when the run's context is canceled or the feed ends.
	Closed() <-chan indicator.Candle
}

// candleStream is the engine's CandleStream. Its Resampler is driven solely by the
// universe's dispatch goroutine; the strategy reads only the atomic History snapshot
// and the Closed channel, so the two goroutines never touch the Resampler's mutable
// state concurrently.
type candleStream struct {
	res  *indicator.Resampler
	out  chan indicator.Candle
	hist atomic.Pointer[indicator.Candles]
}

func (c *candleStream) Closed() <-chan indicator.Candle { return c.out }

func (c *candleStream) History() indicator.Candles {
	if h := c.hist.Load(); h != nil {
		return *h
	}
	return nil
}

// snapshot publishes a copy of the Resampler's history for the strategy goroutine to
// read. Called only from the dispatch goroutine, at seed time and on each bar close.
func (c *candleStream) snapshot() {
	h := slices.Clone(c.res.History())
	c.hist.Store(&h)
}

// Warmup implements Universe.Warmup: it fetches code's historical candles at level
// from the market adapter, seeds a Resampler with them, registers it with the
// universe's tick dispatcher (started on the first call), and returns the live
// stream. See the Universe.Warmup doc for the single-consumer contract.
func (u *universe) Warmup(ctx context.Context, code string, level market.CandleType) (CandleStream, error) {
	if u.market == nil {
		return nil, fmt.Errorf("warmup %s: universe has no market adapter", code)
	}
	compress := level.Duration()
	if compress <= 0 {
		return nil, fmt.Errorf("warmup %s: unknown candle level %d", code, level)
	}
	// Live bars are folded from ticks by fixed-width bucketing (tick.Date truncated
	// to compress), which is epoch-aligned. For daily and longer levels that does not
	// match an exchange/calendar boundary, so the live bars would not continue the
	// seeded historical ones at the same edges. Support intraday levels only.
	if compress >= 24*time.Hour {
		return nil, fmt.Errorf("warmup %s: level %d unsupported — daily and longer bars cannot be aggregated from ticks by fixed-width bucketing; use an intraday level", code, level)
	}
	hist, err := u.market.Candles(ctx, code, level)
	if err != nil {
		return nil, fmt.Errorf("warmup %s: %w", code, err)
	}

	cs := &candleStream{
		res: indicator.NewResampler(compress, indicator.WithSeed(hist)),
		out: make(chan indicator.Candle, 64),
	}

	u.cmu.Lock()
	if u.streams == nil {
		u.streams = map[string][]*candleStream{}
	}
	// Catch up on the code's buffered ticks (those that arrived before this stream
	// existed — a later pairs leg, or another timeframe of a code already streaming).
	// The backlog is left in place so further streams for the code can also catch up.
	// cs is not yet in u.streams, so only this goroutine touches cs.res — folding and
	// the snapshot below are safe under the lock before the dispatcher can reach it.
	for _, tk := range u.pending[code] {
		cs.res.Add(tk)
	}
	cs.snapshot()
	u.streams[code] = append(u.streams[code], cs)
	if !u.started {
		u.started = true
		go u.dispatchCandles(ctx)
	}
	u.cmu.Unlock()
	return cs, nil
}

// warmupBacklog bounds the rolling per-code tick buffer the dispatcher keeps so a
// late-registered stream can catch up, capping memory regardless of how long a run
// streams or how late a stream registers. The oldest ticks beyond it are dropped, so
// a stream registered after more than warmupBacklog ticks have flowed for its code
// catches up only from the most recent ones.
const warmupBacklog = 512

// dispatchCandles is the single consumer of the universe's tick channel once any
// Warmup is in play: it folds each tick into every Resampler registered for the
// tick's code and, when a bar closes, refreshes that stream's history snapshot and
// signals it (dropping the signal if the strategy is behind). It returns — closing
// every stream — when ctx is canceled or the tick feed ends.
func (u *universe) dispatchCandles(ctx context.Context) {
	defer u.closeStreams()
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-u.ticks:
			if !ok {
				return
			}
			u.cmu.Lock()
			// Record every tick in the bounded per-code backlog so a stream registered
			// later can catch up — whether it is a second code (a pairs leg) or another
			// timeframe of a code already streaming. The lock serializes this with
			// registration's replay, so each stream folds a tick exactly once.
			u.bufferLocked(tk)
			streams := slices.Clone(u.streams[tk.Code])
			u.cmu.Unlock()
			for _, cs := range streams {
				closed, ok := cs.res.Add(tk)
				if !ok {
					continue
				}
				cs.snapshot()
				select {
				case cs.out <- closed:
				default: // strategy behind: drop the wakeup, History keeps the bar
				}
			}
		}
	}
}

// bufferLocked appends tk to the rolling backlog for its code, dropping the oldest
// beyond warmupBacklog so it stays bounded. The backlog lets a stream registered
// after some ticks have flowed (a later pairs leg, or another timeframe) catch up.
// Caller holds u.cmu.
func (u *universe) bufferLocked(tk indicator.Tick) {
	if u.pending == nil {
		u.pending = map[string][]indicator.Tick{}
	}
	u.pending[tk.Code] = append(u.pending[tk.Code], tk)
	if p := u.pending[tk.Code]; len(p) > warmupBacklog {
		u.pending[tk.Code] = p[len(p)-warmupBacklog:]
	}
}

func (u *universe) closeStreams() {
	u.cmu.Lock()
	defer u.cmu.Unlock()
	for _, css := range u.streams {
		for _, cs := range css {
			close(cs.out)
		}
	}
}
