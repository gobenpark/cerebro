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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// stubStrategy is an inert strategy: Next blocks until the context is canceled so
// the engine has a goroutine to join on shutdown, and the notify hooks are no-ops.
type stubStrategy struct{}

func (stubStrategy) Next(ctx context.Context, _ *item.Item, _ <-chan indicator.Tick, _ *broker.Broker) {
	<-ctx.Done()
}
func (stubStrategy) NotifyOrder(order.Order) {}
func (stubStrategy) NotifyTrade()            {}
func (stubStrategy) NotifyFund()             {}
func (stubStrategy) Name() string            { return "stub" }

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
	events <- market.ChangeBalanceEvent{Message: "settled", Balance: 50_000}
	var ro <-chan any = events

	mk.EXPECT().AccountPositions().Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance().Return(int64(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(0.0).AnyTimes()
	mk.EXPECT().Events(gomock.Any()).Return(ro).AnyTimes()
	mk.EXPECT().Subscribe(gomock.Any()).Return(nil).AnyTimes()

	c := NewCerebro(
		WithMarket(mk),
		WithStrategy(stubStrategy{}),
		WithTargetItem(&item.Item{Code: "AAA"}),
		WithLogLevel(log.FatalLevel),
	)

	must.Equal(int64(100_000), c.broker.Balance(), "broker seeds its balance from the market")

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	is.Eventually(func() bool {
		return c.broker.Balance() == 50_000
	}, 2*time.Second, 10*time.Millisecond,
		"market balance event must reach the broker through the event engine")

	cancel()
	c.Shutdown()
}
