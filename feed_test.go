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

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"log/slog"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/strategy"
)

// signalStrategy closes done when its Run returns, so a test can observe that the
// run context was canceled (e.g. by a fail-safe shutdown) without reaching into
// Cerebro internals.
type signalStrategy struct {
	name string
	done chan struct{}
	once sync.Once
}

func (s *signalStrategy) Name() string { return s.name }
func (s *signalStrategy) Run(ctx context.Context, _ strategy.Universe, _ broker.Submitter) {
	<-ctx.Done()
	s.once.Do(func() { close(s.done) })
}
func (s *signalStrategy) NotifyOrder(order.Order) {}
func (s *signalStrategy) NotifyTrade()            {}
func (s *signalStrategy) NotifyFund()             {}

// feedMock builds a MockMarket whose Events stream is the supplied channel, with the
// account/commission/subscribe/order calls stubbed so a Cerebro can run against it.
func feedMock(t *testing.T, events <-chan any) *marketmock.MockMarket {
	t.Helper()
	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions(gomock.Any()).Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance(gomock.Any()).Return(decimal.NewFromInt(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(market.Fraction(decimal.Zero)).AnyTimes()
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mk.EXPECT().Events(gomock.Any()).Return(events).AnyTimes()
	return mk
}

// TestCerebro_FeedGuardedReflectsOptions checks the opt-in: feed guarding is off
// unless a timeout or a loss handler is configured.
func TestCerebro_FeedGuardedReflectsOptions(t *testing.T) {
	is := assert.New(t)
	mk := feedMock(t, make(chan any))

	base := []Option{
		WithMarket(mk),
		WithStrategy(stubStrategy{}, "AAA"),
		WithLogger(slog.New(slog.DiscardHandler)),
	}

	is.False(NewCerebro(base...).feedGuarded(), "no feed options means no guard")
	is.True(NewCerebro(append(base, WithFeedTimeout(time.Second))...).feedGuarded(),
		"a feed timeout arms the guard")
	is.True(NewCerebro(append(base, WithFeedLossHandler(func(string) {}))...).feedGuarded(),
		"a loss handler arms the guard")
}

// TestStart_FeedSilenceTriggersDefaultShutdown proves the default fail-safe: with a
// feed timeout but no custom handler, a silent feed trips the watchdog, which Shuts
// the engine down — observed as the strategy's Run returning (the run context was
// canceled).
func TestStart_FeedSilenceTriggersDefaultShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)
	must := require.New(t)

	events := make(chan any) // open but silent: a feed that stalls without closing
	mk := feedMock(t, events)

	sig := &signalStrategy{name: "sig", done: make(chan struct{})}
	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(sig, "AAA"),
		WithFeedTimeout(50*time.Millisecond), // no handler -> default fail-safe Shutdown
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	must.NoError(c.Start(context.Background()))

	select {
	case <-sig.done:
	case <-time.After(2 * time.Second):
		t.Fatal("default fail-safe should shut the engine down on a stale feed")
	}
	c.Shutdown() // idempotent; blocks on the in-flight shutdown so goleak sees a clean drain
}

// TestStart_FeedSilenceInvokesCustomHandler proves a configured handler receives the
// staleness signal and that it replaces the default shutdown — the engine keeps
// running until the test cancels it.
func TestStart_FeedSilenceInvokesCustomHandler(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)

	events := make(chan any)
	mk := feedMock(t, events)

	reasons := make(chan string, 4)
	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}, "AAA"),
		WithFeedTimeout(50*time.Millisecond),
		WithFeedLossHandler(func(reason string) { reasons <- reason }),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	select {
	case r := <-reasons:
		is.Contains(r, "no market data", "handler reason should describe the staleness")
	case <-time.After(2 * time.Second):
		t.Fatal("a stale feed should invoke the feed-loss handler")
	}

	cancel()
	c.Shutdown()
}

// TestStart_FeedChannelCloseInvokesHandler proves that a handler alone (no timeout)
// enables close-as-loss detection: closing the stream while the run is live is
// reported as feed loss.
func TestStart_FeedChannelCloseInvokesHandler(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)

	events := make(chan any)
	mk := feedMock(t, events)

	reasons := make(chan string, 4)
	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}, "AAA"),
		WithFeedLossHandler(func(reason string) { reasons <- reason }), // guard on, no timeout
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	close(events) // the adapter ends the feed while the run is still live

	select {
	case r := <-reasons:
		is.Contains(r, "channel closed", "handler reason should describe the close")
	case <-time.After(2 * time.Second):
		t.Fatal("a mid-run channel close should invoke the feed-loss handler")
	}

	cancel()
	c.Shutdown()
}

// TestStart_FeedWatchdogResetByTicks proves the watchdog does not false-positive
// while data flows: a steady tick stream keeps it armed-but-quiet for several timeout
// windows, and only once ticks stop does it trip.
func TestStart_FeedWatchdogResetByTicks(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)

	events := make(chan any)
	mk := feedMock(t, events)

	reasons := make(chan string, 4)
	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}, "AAA"),
		WithFeedTimeout(100*time.Millisecond),
		WithFeedLossHandler(func(reason string) { reasons <- reason }),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	stop := make(chan struct{})
	streamed := make(chan struct{})
	go func() {
		defer close(streamed)
		tk := time.NewTicker(25 * time.Millisecond) // well under the 100ms timeout
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				select {
				case events <- indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(10)}:
				case <-stop:
					return
				}
			}
		}
	}()

	// No trip across ~300ms (3x the timeout) of steady ticks.
	select {
	case r := <-reasons:
		t.Fatalf("watchdog tripped while ticks were flowing: %s", r)
	case <-time.After(300 * time.Millisecond):
	}

	// Stop the stream; with no further data the watchdog must trip.
	close(stop)
	<-streamed
	select {
	case r := <-reasons:
		is.Contains(r, "no market data")
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog should trip once the data stops")
	}

	cancel()
	c.Shutdown()
}

// TestStart_FeedHeartbeatResetsWatchdog proves a FeedStatusEvent acts as a heartbeat:
// during a tickless quiet period (e.g. a reconnect), status events alone keep the
// watchdog from tripping.
func TestStart_FeedHeartbeatResetsWatchdog(t *testing.T) {
	defer goleak.VerifyNone(t)
	must := require.New(t)

	events := make(chan any)
	mk := feedMock(t, events)

	reasons := make(chan string, 4)
	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}, "AAA"),
		WithFeedTimeout(100*time.Millisecond),
		WithFeedLossHandler(func(reason string) { reasons <- reason }),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	stop := make(chan struct{})
	streamed := make(chan struct{})
	go func() {
		defer close(streamed)
		tk := time.NewTicker(25 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				select {
				case events <- market.FeedStatusEvent{State: market.FeedConnected, Message: "heartbeat"}:
				case <-stop:
					return
				}
			}
		}
	}()

	// Heartbeats with no ticks must still hold the watchdog off for >3x the timeout.
	select {
	case r := <-reasons:
		t.Fatalf("watchdog tripped despite heartbeats: %s", r)
	case <-time.After(300 * time.Millisecond):
	}

	close(stop)
	<-streamed
	cancel()
	c.Shutdown()
}

// TestStart_UnguardedChannelCloseIsGraceful proves the default (unguarded) behavior
// is unchanged: a channel close ends the pump without shutting the engine down, so a
// backtest that exhausts its data is not mistaken for a feed loss.
func TestStart_UnguardedChannelCloseIsGraceful(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)

	events := make(chan any)
	mk := feedMock(t, events)

	sig := &signalStrategy{name: "sig", done: make(chan struct{})}
	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(sig, "AAA"),
		WithLogger(slog.New(slog.DiscardHandler)), // no feed options -> unguarded
	)
	is.False(c.feedGuarded())

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	close(events) // normal end of stream for an unguarded feed

	// The engine must stay up: no fail-safe shutdown, so the strategy keeps running.
	select {
	case <-sig.done:
		t.Fatal("an unguarded channel close must not shut the engine down")
	case <-time.After(200 * time.Millisecond):
	}

	// Only an explicit cancel stops it.
	cancel()
	select {
	case <-sig.done:
	case <-time.After(2 * time.Second):
		t.Fatal("strategy should stop after an explicit cancel")
	}
	c.Shutdown()
}
