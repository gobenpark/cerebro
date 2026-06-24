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
package indicator_test

import (
	"slices"
	"testing"
	"time"

	"github.com/gobenpark/cerebro/indicator"
)

var fuzzCompress = []time.Duration{
	time.Minute,
	30 * time.Second,
	2 * time.Minute,
	time.Hour,
}

// decodeTicks derives a deterministic, time-ordered tick series and a bucket width
// from arbitrary fuzz bytes. The first byte selects the compress width; the rest are
// consumed three at a time as (time delta in seconds, price, volume). Deltas are
// non-negative, so timestamps are non-decreasing — the order a Resampler assumes.
func decodeTicks(data []byte) (time.Duration, []indicator.Tick) {
	if len(data) == 0 {
		return time.Minute, nil
	}
	compress := fuzzCompress[int(data[0])%len(fuzzCompress)]

	const maxTicks = 4096 // bound work so a huge corpus entry cannot stall the fuzzer
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	cur := base

	var ticks []indicator.Tick
	body := data[1:]
	for i := 0; i+3 <= len(body) && len(ticks) < maxTicks; i += 3 {
		cur = cur.Add(time.Duration(body[i]) * time.Second)
		ticks = append(ticks, indicator.Tick{
			Date:   cur,
			Code:   "AAA",
			Price:  dec(int64(body[i+1]) + 1), // 1..256, never zero
			Volume: int64(body[i+2]),
		})
	}
	return compress, ticks
}

// FuzzResamplerMatchesResample generalizes the fixed cross-check
// (TestResampler_MatchesResample) over arbitrary ordered tick series: feeding ticks
// one at a time through a Resampler must yield exactly the candles the batch Resample
// produces, so the two bucketing/folding paths can never drift — including on
// equal-timestamp ticks, exact boundaries, and time gaps the fuzzer will explore.
func FuzzResamplerMatchesResample(f *testing.F) {
	f.Add([]byte{0, 10, 100, 5, 40, 110, 1, 90, 95, 3}) // two same-bucket folds then a gap
	f.Add([]byte{1, 0, 50, 2, 0, 60, 1})                // equal-timestamp ticks (delta 0)
	f.Add([]byte{2})                                    // compress only, no ticks
	f.Add([]byte{})                                     // empty input

	f.Fuzz(func(t *testing.T, data []byte) {
		compress, ticks := decodeTicks(data)

		batch := indicator.Resample(ticks, compress)

		r := indicator.NewResampler(compress)
		for _, tk := range ticks {
			r.Add(tk)
		}
		// "All bars" from the incremental path = closed history plus the forming bar.
		live := slices.Clone(r.History())
		if cur, ok := r.Current(); ok {
			live = append(live, &cur)
		}

		if len(live) != len(batch) {
			t.Fatalf("candle count mismatch: batch=%d incremental=%d (ticks=%d, compress=%s)",
				len(batch), len(live), len(ticks), compress)
		}
		for i := range batch {
			eqCandle(t, batch[i], live[i])
		}
	})
}

func benchTicks(n int) []indicator.Tick {
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	ticks := make([]indicator.Tick, n)
	for i := range ticks {
		ticks[i] = indicator.Tick{
			Date:   base.Add(time.Duration(i) * time.Second),
			Code:   "AAA",
			Price:  dec(int64(100 + i%50)),
			Volume: 1,
		}
	}
	return ticks
}

// BenchmarkResample measures the batch path over a minute-bucketed day of ticks.
func BenchmarkResample(b *testing.B) {
	ticks := benchTicks(10_000)
	b.ReportAllocs()
	for b.Loop() {
		_ = indicator.Resample(ticks, time.Minute)
	}
}

// BenchmarkResamplerAdd measures the per-tick incremental path with a bounded window,
// the live-strategy hot path. The decimal price is built once outside the loop so the
// measurement isolates Add's own work (bucket fold/close) rather than decimal
// construction; only the timestamp advances, keeping bars closing at a realistic rate.
func BenchmarkResamplerAdd(b *testing.B) {
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	r := indicator.NewResampler(time.Minute, indicator.WithWindow(256))
	tk := indicator.Tick{Code: "AAA", Price: dec(100), Volume: 1}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		tk.Date = base.Add(time.Duration(i) * time.Second)
		r.Add(tk)
		i++
	}
}
