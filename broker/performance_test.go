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

package broker_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
)

// ledgerStore is a Storage that loads a fixed ledger, to exercise restore paths.
type ledgerStore struct{ l broker.Ledger }

func (s ledgerStore) Save(context.Context, broker.Ledger) error   { return nil }
func (s ledgerStore) Load(context.Context) (broker.Ledger, error) { return s.l, nil }

// TestBroker_RecordsClosedTrade verifies a round-trip is logged only when the lot
// returns to flat, with the right entry/exit/PnL and the tick clock's open/close times.
func TestBroker_RecordsClosedTrade(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0) // no commission
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()
	day0 := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	day1 := day0.AddDate(0, 0, 1)

	// A tick sets the broker clock that stamps the opening fill.
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(100), Date: day0})
	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10))
	is.Empty(bk.Trades(), "an open position is not a closed trade yet")

	// Advance the clock, then close the position.
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(120), Date: day1})
	sell := sellLimit("AAA", 10, 120)
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 120, 10))

	trades := bk.Trades()
	must.Len(trades, 1)
	tr := trades[0]
	is.Equal("alpha", tr.Strategy)
	is.Equal("AAA", tr.Code)
	eqDec(t, 10, tr.Qty)
	eqDec(t, 100, tr.Entry)
	eqDec(t, 120, tr.Exit)
	eqDec(t, 200, tr.Realized, "(120-100)*10")
	eqDec(t, 200, tr.NetPnL(), "no commission")
	is.True(tr.OpenedAt.Equal(day0))
	is.True(tr.ClosedAt.Equal(day1))
}

// TestBroker_EquitySampledDaily verifies equity is sampled once per calendar day from
// the tick clock, and that equity marks open positions to the latest tick.
func TestBroker_EquitySampledDaily(t *testing.T) {
	must := require.New(t)

	bk, _ := newBrokerUnderTest(t, 100_000, 0)
	ctx := context.Background()
	day0 := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	// Two ticks the same day yield one sample; later days add one each.
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(100), Date: day0})
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(101), Date: day0.Add(time.Hour)})
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(102), Date: day0.AddDate(0, 0, 1)})

	curve := bk.EquityCurve()
	must.Len(curve, 2, "one sample per day (day0, day1)")
	for _, p := range curve {
		eqDec(t, 100_000, p.Equity, "no positions: equity equals settled cash")
	}
}

// TestBroker_EquityMarksOpenPosition verifies Equity is settled cash plus the
// mark-to-market value of an open position at the latest tick price.
func TestBroker_EquityMarksOpenPosition(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(100), Date: time.Unix(0, 0)})
	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10))

	// Price moves to 110: equity = cash 100_000 + 10 * 110 (balance is unchanged here
	// because this harness emits no settlement event).
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(110), Date: time.Unix(0, 0)})
	eqDec(t, 101_100, bk.Equity(), "cash + 10*110")
}

// TestBroker_EquitySameDayReflectsLatest guards that a second tick on the same day
// updates that day's equity point to the latest (closing) value rather than keeping
// the day-open one, so intraday moves are not dropped from the curve.
func TestBroker_EquitySameDayReflectsLatest(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()
	day0 := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)

	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(100), Date: day0})
	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10))

	// A later same-day tick at 110 must move the day's equity to cash + 10*110.
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(110), Date: day0.Add(2 * time.Hour)})

	curve := bk.EquityCurve()
	must.Len(curve, 1, "all ticks share one day")
	eqDec(t, 101_100, curve[0].Equity, "day point reflects the latest price, not the open")
}

// TestBroker_RestoredLotClosesToSaneTrade guards that a position restored from the
// ledger, when later closed, emits a Trade with the correct quantity, entry, and
// realized PnL (the round-trip accumulators are seeded from the restored lot).
func TestBroker_RestoredLotClosesToSaneTrade(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	bk.SetStorage(ledgerStore{l: broker.Ledger{
		Version: 1,
		Lots: []broker.LotState{{
			Strategy: "alpha", Item: &item.Item{Code: "AAA"},
			Size: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), Peak: decimal.NewFromInt(100),
		}},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
	}})
	must.NoError(bk.Restore(ctx))

	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(120), Date: time.Unix(0, 0)})
	sell := sellLimit("AAA", 10, 120)
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 120, 10))

	trades := bk.Trades()
	must.Len(trades, 1)
	eqDec(t, 10, trades[0].Qty, "quantity from the restored lot, not zero")
	eqDec(t, 100, trades[0].Entry, "entry = cost/size of the restored lot")
	eqDec(t, 120, trades[0].Exit)
	eqDec(t, 200, trades[0].Realized, "(120-100)*10")
	is.Equal("AAA", trades[0].Code)
}
