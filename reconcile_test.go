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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"log/slog"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/risk"
	"github.com/gobenpark/cerebro/strategy"
)

// chanScreener is a screener whose snapshots a test pushes onto ch.
type chanScreener struct{ ch chan []*item.Item }

func (s chanScreener) Screen(_ context.Context) <-chan []*item.Item { return s.ch }

// spawnedStrategy signals when its Run starts and when it returns, so a test can
// observe a per-item strategy being spawned and later evicted.
type spawnedStrategy struct {
	code    string
	started chan struct{}
	stopped chan struct{}
}

func (s *spawnedStrategy) Name() string { return "scan:" + s.code }
func (s *spawnedStrategy) Run(ctx context.Context, _ strategy.Universe, _ broker.Submitter) {
	close(s.started)
	<-ctx.Done()
	close(s.stopped)
}
func (s *spawnedStrategy) NotifyOrder(order.Order) {}

// TestStart_ScreenerSpawnsAndEvictsDynamically drives the whole dynamic-screening
// path: a screened code spawns a per-item strategy, and dropping it (flat) evicts the
// strategy under the default KeepUntilFlat policy — observed via each instance's Run
// starting and returning.
func TestStart_ScreenerSpawnsAndEvictsDynamically(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)

	events := make(chan any) // open and silent
	mk := feedMock(t, events)

	var mu sync.Mutex
	instances := map[string]*spawnedStrategy{}
	factory := func(it *item.Item) strategy.Strategy {
		s := &spawnedStrategy{code: it.Code, started: make(chan struct{}), stopped: make(chan struct{})}
		mu.Lock()
		instances[it.Code] = s
		mu.Unlock()
		return s
	}
	get := func(code string) *spawnedStrategy {
		mu.Lock()
		defer mu.Unlock()
		return instances[code]
	}

	sc := chanScreener{ch: make(chan []*item.Item)}
	c := NewCerebro(
		WithMarket(mk),
		WithScreener(sc, factory),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(c.Start(ctx))

	// Screen AAA -> a strategy is spawned for it.
	sc.ch <- []*item.Item{{Code: "AAA"}}
	is.Eventually(func() bool {
		s := get("AAA")
		if s == nil {
			return false
		}
		select {
		case <-s.started:
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond, "a screened code must spawn a per-item strategy")

	aaa := get("AAA")

	// Drop AAA. Its position is flat (it placed no orders), so KeepUntilFlat evicts it
	// and its Run returns.
	sc.ch <- []*item.Item{}
	select {
	case <-aaa.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("a dropped, flat code must be evicted and its strategy stopped")
	}

	cancel()
	c.Shutdown()
}

// TestStart_AllowsRiskPolicyForScreenerStrategy guards that an enabled WithRiskPolicy
// naming a strategy the screener factory will produce is NOT rejected at Start — its
// name is unknown until the reconciler spawns it, so validatePolicies must defer to
// the reconciler (which applies the override) when a screener is configured.
func TestStart_AllowsRiskPolicyForScreenerStrategy(t *testing.T) {
	defer goleak.VerifyNone(t)
	must := require.New(t)

	events := make(chan any)
	mk := feedMock(t, events)
	sc := chanScreener{ch: make(chan []*item.Item)}

	c := NewCerebro(
		WithMarket(mk),
		WithScreener(sc, func(it *item.Item) strategy.Strategy {
			return &spawnedStrategy{code: it.Code, started: make(chan struct{}), stopped: make(chan struct{})}
		}),
		// "scan:AAA" is produced by the factory only when AAA is screened in.
		WithRiskPolicy("scan:AAA", risk.Policy{StopLoss: 0.05}),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	must.NoError(c.Start(ctx), "an enabled risk policy for a screener-spawned strategy must not be rejected")

	cancel()
	c.Shutdown()
}
