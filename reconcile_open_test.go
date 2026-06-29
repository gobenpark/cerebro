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

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"log/slog"

	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// reportingMarket is a MockMarket that also satisfies market.OpenOrderReporter, so a
// Start test can assert Cerebro recovers the exchange's resting orders on startup.
type reportingMarket struct {
	*marketmock.MockMarket
	open []order.Order
	err  error
}

func (m *reportingMarket) OpenOrders(context.Context) ([]order.Order, error) {
	return m.open, m.err
}

// TestStart_RecoversOpenOrdersFromExchange verifies Start wires reconciliation in: a
// resting order the exchange still has working is recovered into the broker's open
// set and re-reserves its cash, so a restart mid-order does not forget it.
func TestStart_RecoversOpenOrdersFromExchange(t *testing.T) {
	defer goleak.VerifyNone(t)

	is := assert.New(t)
	must := require.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions(gomock.Any()).Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance(gomock.Any()).Return(decimal.NewFromInt(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(market.Fraction(decimal.Zero)).AnyTimes()
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ev := make(chan any)
	var ro <-chan any = ev
	mk.EXPECT().Events(gomock.Any()).Return(ro).AnyTimes()

	buy := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, decimal.NewFromInt(10), decimal.NewFromInt(100))
	buy.SetID("EX-1") // exchange id, so later order events match it
	rm := &reportingMarket{MockMarket: mk, open: []order.Order{buy}}

	c := NewCerebro(
		WithMarket(rm),
		WithStrategy(stubStrategy{}, "AAA"),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	is.True(decimal.NewFromInt(100_000).Equal(c.broker.Available()),
		"before Start, the exchange's working orders are not yet recovered")

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(c.Start(ctx))

	is.True(decimal.NewFromInt(99_000).Equal(c.broker.Available()),
		"Start must recover the resting buy and reserve its 1000")
	must.Len(c.broker.Orders("AAA"), 1, "the recovered order is in the open set")

	cancel()
	c.Shutdown()
}

// TestStart_FailsWhenOpenOrderRecoveryFails verifies an OpenOrders failure aborts
// Start: trading must not begin while Cerebro is blind to the exchange's working
// orders. The failure is a clean pre-spawn abort, so the instance stays retryable.
func TestStart_FailsWhenOpenOrderRecoveryFails(t *testing.T) {
	must := require.New(t)
	is := assert.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions(gomock.Any()).Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance(gomock.Any()).Return(decimal.NewFromInt(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(market.Fraction(decimal.Zero)).AnyTimes()
	rm := &reportingMarket{MockMarket: mk, err: errors.New("exchange down")}

	c := NewCerebro(
		WithMarket(rm),
		WithStrategy(stubStrategy{}, "AAA"),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	err := c.Start(context.Background())
	must.ErrorContains(err, "exchange down")
	is.False(c.started, "a failed open-order recovery must not latch the one-shot guard")
}
