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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
)

// TestResampler_WithSeedWarmsHistory verifies a seeded Resampler reports its
// history before any tick, that the first live tick opens a forming bar without
// disturbing the seed, and that a closed live bar is appended after the seed.
func TestResampler_WithSeedWarmsHistory(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	seed := indicator.Candles{
		{Date: base, Code: "AAA", Open: dec(10), High: dec(10), Low: dec(10), Close: dec(10), Volume: 1},
		{Date: base.Add(time.Minute), Code: "AAA", Open: dec(11), High: dec(11), Low: dec(11), Close: dec(11), Volume: 1},
	}
	r := indicator.NewResampler(time.Minute, indicator.WithSeed(seed))
	must.Len(r.History(), 2) // warm at construction, before any tick

	// The first live tick opens the forming bar; the seed is untouched.
	_, ok := r.Add(indicator.Tick{Date: base.Add(2 * time.Minute), Code: "AAA", Price: dec(12), Volume: 1})
	is.False(ok)
	must.Len(r.History(), 2)

	// A tick in the next bucket closes the live bar, appended after the seed.
	closed, ok := r.Add(indicator.Tick{Date: base.Add(3 * time.Minute), Code: "AAA", Price: dec(13), Volume: 1})
	is.True(ok)
	is.True(closed.Close.Equal(dec(12)), "the closed bar holds only the first live tick")
	must.Len(r.History(), 3) // seed(2) + one closed live bar
}

// TestResampler_WithSeedRespectsWindow verifies a seed larger than the window is
// trimmed to the most recent bars at construction, regardless of option order.
func TestResampler_WithSeedRespectsWindow(t *testing.T) {
	is := assert.New(t)
	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	seed := make(indicator.Candles, 5)
	for i := range seed {
		seed[i] = &indicator.Candle{Date: base.Add(time.Duration(i) * time.Minute), Close: dec(int64(i))}
	}
	r := indicator.NewResampler(time.Minute, indicator.WithSeed(seed), indicator.WithWindow(3))

	h := r.History()
	is.Len(h, 3)                      // trimmed to the window
	is.True(h[0].Close.Equal(dec(2))) // kept the most recent 3 (indices 2,3,4)
	is.True(h[2].Close.Equal(dec(4)))
}
