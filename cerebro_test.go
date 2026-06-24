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
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"log/slog"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/strategy"
)

// stubStrategy is an inert strategy: Run blocks until the context is canceled so
// the engine has a goroutine to join on shutdown, and the notify hooks are no-ops.
type stubStrategy struct{}

func (stubStrategy) Run(ctx context.Context, _ strategy.Universe, _ broker.Submitter) {
	<-ctx.Done()
}
func (stubStrategy) NotifyOrder(order.Order) {}
func (stubStrategy) NotifyTrade()            {}
func (stubStrategy) NotifyFund()             {}
func (stubStrategy) Name() string            { return "stub" }

// flakyStorage fails Load on the first call and succeeds (empty ledger) after,
// to exercise a retried Start following a transient restore failure.
type flakyStorage struct{ loads int }

func (f *flakyStorage) Save(context.Context, broker.Ledger) error { return nil }
func (f *flakyStorage) Load(context.Context) (broker.Ledger, error) {
	f.loads++
	if f.loads == 1 {
		return broker.Ledger{}, errors.New("transient restore failure")
	}
	return broker.Ledger{}, nil
}

// TestStart_RetryAfterFailedRestoreDoesNotDuplicateEngine guards the fix where a
// failed restore left a strategy engine appended to c.engines: retrying Start
// would then run two engines, double-subscribing and double-spawning. The engine
// is built only after restore succeeds and assigned (not appended), so a retry
// leaves exactly one.
func TestStart_RetryAfterFailedRestoreDoesNotDuplicateEngine(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions().Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance().Return(decimal.NewFromInt(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(decimal.Zero).AnyTimes()
	mk.EXPECT().Subscribe(gomock.Any()).Return(nil).AnyTimes()
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ev := make(chan any)
	var ro <-chan any = ev
	mk.EXPECT().Events(gomock.Any()).Return(ro).AnyTimes()

	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}),
		WithTargetItem(&item.Item{Code: "AAA"}),
		WithStorage(&flakyStorage{}),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	// First Start fails inside Restore, before the engine is built.
	must.Error(c.Start(context.Background()))
	is.Empty(c.engines, "a failed restore must leave no engine behind")

	// Retry succeeds and must run exactly one engine.
	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))
	is.Len(c.engines, 1, "retry after a failed restore must not duplicate the engine")

	cancel()
	c.Shutdown()
}

// TestStart_WiresBrokerAsEventListener guards the fix where the broker was built
// but never registered with the event engine, so market events (balance/order
// changes) never reached it. Here the market emits ChangeBalanceEvent; the broker
// must observe the new settled balance, which only happens if Start registers it.
func TestStart_WiresBrokerAsEventListener(t *testing.T) {
	defer goleak.VerifyNone(t)

	is := assert.New(t)
	must := require.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)

	// A single event, queued before Start, exercises the ordering guarantee: the
	// broker is registered synchronously before the market-events pump runs, so
	// even one immediate event must reach it. (An asynchronous registration could
	// race ahead of this lone event and silently drop it.)
	events := make(chan any, 1)
	events <- market.ChangeBalanceEvent{Message: "settled", Balance: decimal.NewFromInt(50_000)}
	var ro <-chan any = events

	mk.EXPECT().AccountPositions().Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance().Return(decimal.NewFromInt(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(decimal.Zero).AnyTimes()
	mk.EXPECT().Events(gomock.Any()).Return(ro).AnyTimes()
	mk.EXPECT().Subscribe(gomock.Any()).Return(nil).AnyTimes()

	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}),
		WithTargetItem(&item.Item{Code: "AAA"}),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	must.True(decimal.NewFromInt(100_000).Equal(c.broker.Balance()), "broker seeds its balance from the market")

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	is.Eventually(func() bool {
		return c.broker.Balance().Equal(decimal.NewFromInt(50_000))
	}, 2*time.Second, 10*time.Millisecond,
		"market balance event must reach the broker through the event engine")

	cancel()
	c.Shutdown()
}
