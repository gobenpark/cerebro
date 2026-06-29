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
	"errors"
	"log/slog"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	eventmock "github.com/gobenpark/cerebro/event/mock"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/store"
)

// reportingMarket is a MockMarket that additionally satisfies
// market.OpenOrderReporter, so a reconcile test can hand the broker a snapshot of the
// resting orders the exchange still has working.
type reportingMarket struct {
	*marketmock.MockMarket
	orders []order.Order
	err    error
}

func (m *reportingMarket) OpenOrders(context.Context) ([]order.Order, error) {
	return m.orders, m.err
}

// newReportingBroker builds a broker over a reportingMarket seeded with balance and
// the resting orders OpenOrders reports (err is returned instead when non-nil). The
// returned MockMarket lets a test add an Order()/Cancel() expectation.
func newReportingBroker(t *testing.T, balance int64, orders []order.Order, err error) (*broker.Broker, *marketmock.MockMarket) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions(gomock.Any()).Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance(gomock.Any()).Return(decimal.NewFromInt(balance)).AnyTimes()
	mk.EXPECT().Commission().Return(market.Percent(decimal.NewFromFloat(0))).AnyTimes()

	bc := eventmock.NewMockBroadcaster(ctrl)
	bc.EXPECT().BroadCast(gomock.Any()).AnyTimes()
	bc.EXPECT().BroadCastContext(gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	rm := &reportingMarket{MockMarket: mk, orders: orders, err: err}
	return broker.NewDefaultBroker(bc, rm, slog.New(slog.DiscardHandler)), mk
}

// TestReconcileOpenOrders_SeedsOpenSetAndReservation verifies a resting order the
// exchange still holds is recovered into the open set and re-reserves its cash, so
// Available reflects it after a restart.
func TestReconcileOpenOrders_SeedsOpenSetAndReservation(t *testing.T) {
	must := require.New(t)

	buy := buyLimit("AAA", 10, 100) // value 1000
	buy.SetID("EX-1")
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)

	eqDec(t, 100_000, bk.Available(), "nothing is reserved before reconcile")

	must.NoError(bk.ReconcileOpenOrders(context.Background()))

	eqDec(t, 99_000, bk.Available(), "the recovered open buy reserves its 1000")
	must.Len(bk.Orders("AAA"), 1, "the recovered order is in the open set")
	must.Equal("EX-1", bk.Orders("AAA")[0].ID())
}

// TestReconcileOpenOrders_RecoveredOrderIsCancelable proves a recovered order is fully
// tracked, not just present: it is marked submitted, so a cancel takes the direct path
// to the market (rather than deferring forever as an unknown order the market will
// never re-submit), and the confirmation releases its reservation.
func TestReconcileOpenOrders_RecoveredOrderIsCancelable(t *testing.T) {
	must := require.New(t)

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-5")
	bk, mk := newReportingBroker(t, 100_000, []order.Order{buy}, nil)
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).Return(nil)
	ctx := context.Background()

	must.NoError(bk.ReconcileOpenOrders(ctx))
	must.NoError(bk.Cancel(ctx, "EX-5"))
	bk.Listen(ctx, market.ChangeOrderEvent{ID: "EX-5", Action: order.Canceled, Message: "canceled"})

	eqDec(t, 100_000, bk.Available(), "cancel confirmation releases the recovered order's reservation")
	must.Empty(bk.Orders("AAA"))
}

// TestReconcileOpenOrders_UnobservedPartialBooksFullOnComplete verifies that without
// restored fill progress (storage off, or a partial that filled offline), a recovered
// order has no observed/booked quantity, so its completion books the FULL size — the
// offline fill is not silently dropped. The reservation still tracks only the exchange's
// unfilled remainder.
func TestReconcileOpenOrders_UnobservedPartialBooksFullOnComplete(t *testing.T) {
	must := require.New(t)

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-2")
	buy.Partial(decimal.NewFromInt(6)) // exchange reports 6/10 filled; the broker never observed it
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)
	ctx := context.Background()

	must.NoError(bk.ReconcileOpenOrders(ctx))
	eqDec(t, 99_600, bk.Available(), "reservation tracks the exchange's 4-unit remainder")

	// No observed progress was restored, so the completion books the full size (10), not
	// just the 4-unit remainder — the 6 that filled while the broker was unaware are kept.
	bk.Listen(ctx, completedFill(buy, 100, 4))

	pos, _, ok := bk.StrategyPosition("", "AAA") // unattributed: no restored lot to own it
	must.True(ok, "the completed order books a position")
	eqDec(t, 10, pos.Size, "with no observed progress the completion books the full size")
	eqDec(t, 100_000, bk.Available(), "the completed order releases its reservation")
}

// TestReconcileOpenOrders_PartialFillWithStorageNoDoubleCount is the storage-on (live
// recommended) case: a partial fill observed and persisted before the crash must not
// be counted twice when the working order is recovered and later completes. The
// restored lot holds the filled portion and the recovered order's completion books
// only the remainder, so total tracked exposure equals the exchange's — not the sum of
// both. (Before the fill-progress seeding, the completion booked the full size on top
// of the restored lot, double-counting it.)
func TestReconcileOpenOrders_PartialFillWithStorageNoDoubleCount(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	// A 6-of-10 partial fill was observed and persisted under strategy "alpha".
	mem := store.NewMemoryStorage()
	must.NoError(mem.Save(ctx, broker.Ledger{
		Version: 2,
		Lots: []broker.LotState{{
			Strategy: "alpha", Item: &item.Item{Code: "AAA"},
			Size: decimal.NewFromInt(6), Cost: decimal.NewFromInt(600), Peak: decimal.NewFromInt(100),
		}},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
		// The 6-unit partial was observed and persisted together with the lot.
		Filled: map[string]decimal.Decimal{"EX-7": decimal.NewFromInt(6)},
	}))

	// The same order is still working on the exchange with 4 of 10 remaining.
	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-7")
	buy.Partial(decimal.NewFromInt(6))
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)
	bk.SetStorage(mem)

	must.NoError(bk.Restore(ctx))             // restores the persisted 6-unit lot
	must.NoError(bk.ReconcileOpenOrders(ctx)) // recovers the working order, seeds filled=6

	bk.Listen(ctx, completedFill(buy, 100, 4)) // completion books only the 4-unit remainder

	// The persisted 6 is not re-counted: total exposure is 10 units (alpha's restored 6
	// plus the recovered buy's 4), equity 101000 — not 16 units / 101600. (The recovered
	// buy stays unattributed — a buy is never credited to an existing holder — so the 4
	// lands in the unattributed lot; the no-double-count invariant is the equity total.)
	eqDec(t, 101_000, bk.Equity(), "persisted partial not re-counted: 10 units total, not 16")
}

// TestReconcileOpenOrders_RecoveredBuyNotStolenFromAnotherStrategy guards that the
// sole-holder attribution does NOT apply to buys: a buy on a code another strategy holds
// may belong to a different strategy opening its own position, so crediting it to the
// holder would contaminate that lot. The recovered buy stays unattributed and the
// holder's lot is untouched.
func TestReconcileOpenOrders_RecoveredBuyNotStolenFromAnotherStrategy(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	// alpha holds 10 AAA (persisted). beta had a resting BUY for AAA across the restart.
	mem := store.NewMemoryStorage()
	must.NoError(mem.Save(ctx, broker.Ledger{
		Version: 2,
		Lots: []broker.LotState{{
			Strategy: "alpha", Item: &item.Item{Code: "AAA"},
			Size: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), Peak: decimal.NewFromInt(100),
		}},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
	}))

	buy := buyLimit("AAA", 5, 100) // beta's buy, unattributed from the exchange
	buy.SetID("EX-BETA")
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)
	bk.SetStorage(mem)

	must.NoError(bk.Restore(ctx))
	must.NoError(bk.ReconcileOpenOrders(ctx))

	bk.Listen(ctx, completedFill(buy, 100, 5))

	pos, _, ok := bk.StrategyPosition("alpha", "AAA")
	must.True(ok)
	eqDec(t, 10, pos.Size, "alpha keeps exactly its 10 — the foreign buy is not stolen into its lot")
}

// TestReconcileOpenOrders_OfflinePartialBooksUnobservedShares is the mixed case: the
// broker observed and persisted part of a fill, then more filled while it was down. On
// recovery the completion must book everything past what was observed — the offline
// shares AND the still-open remainder — onto the restored lot, never dropping the
// offline shares.
func TestReconcileOpenOrders_OfflinePartialBooksUnobservedShares(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	// alpha observed 3 of a 10-share buy and persisted that (lot 3, filled 3). Then 3
	// more filled offline (exchange shows 6/10, 4 remaining) — unobserved, so the
	// persisted progress still reads 3.
	mem := store.NewMemoryStorage()
	must.NoError(mem.Save(ctx, broker.Ledger{
		Version: 2,
		Lots: []broker.LotState{{
			Strategy: "alpha", Item: &item.Item{Code: "AAA"},
			Size: decimal.NewFromInt(3), Cost: decimal.NewFromInt(300), Peak: decimal.NewFromInt(100),
		}},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
		Filled:   map[string]decimal.Decimal{"EX-OFF": decimal.NewFromInt(3)},
	}))

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-OFF")
	buy.Partial(decimal.NewFromInt(6)) // exchange: 6/10 filled (3 observed + 3 offline)
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)
	bk.SetStorage(mem)

	must.NoError(bk.Restore(ctx))
	must.NoError(bk.ReconcileOpenOrders(ctx))

	bk.Listen(ctx, completedFill(buy, 100, 4))

	// fillInc = size(10) - observed(3) = 7 → the 3 offline + 4 remainder book (to the
	// unattributed lot, since buys are not auto-attributed) on top of alpha's restored 3:
	// total 10 units, equity 101000. Dropping the offline shares would show 7 units
	// (100700); double-counting the observed 3 would show 13 units (101300).
	eqDec(t, 101_000, bk.Equity(), "offline + remainder both book; observed 3 neither dropped nor double-counted")
}

// TestReconcileOpenOrders_RecoveredSellClosesRestoredLot guards the duplicate-exit
// hazard: a strategy that holds a restored position and had a resting exit (sell) order
// on the exchange must see that lot CLOSED when the recovered sell fills — otherwise the
// broker thinks the position is still open and a reactive exit could sell it again. The
// recovered sell is unattributed from the exchange; reconciliation attributes it to the
// sole holder of its code so its fill reduces the right lot.
func TestReconcileOpenOrders_RecoveredSellClosesRestoredLot(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	// alpha holds 10 shares (persisted) with a resting take-profit sell on the exchange.
	mem := store.NewMemoryStorage()
	must.NoError(mem.Save(ctx, broker.Ledger{
		Version: 2,
		Lots: []broker.LotState{{
			Strategy: "alpha", Item: &item.Item{Code: "AAA"},
			Size: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), Peak: decimal.NewFromInt(100),
		}},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
	}))

	sell := sellLimit("AAA", 10, 120) // unattributed from the exchange
	sell.SetID("EX-SELL")
	bk, _ := newReportingBroker(t, 100_000, []order.Order{sell}, nil)
	bk.SetStorage(mem)

	must.NoError(bk.Restore(ctx))             // restores alpha's 10-unit lot
	must.NoError(bk.ReconcileOpenOrders(ctx)) // attributes the sell to alpha (sole AAA owner)

	bk.Listen(ctx, completedFill(sell, 120, 10)) // the exit fills

	_, _, ok := bk.StrategyPosition("alpha", "AAA")
	must.False(ok, "the recovered sell closes alpha's lot — no phantom position to re-exit")
}

// TestReconcileOpenOrders_DropsInconsistentFillProgress guards the defensive path for an
// adapter that violates the Size()=original contract by reporting Size() as the remainder:
// restored fill progress that exceeds Size() is dropped so a completion still books the
// reported remainder, instead of a negative increment that would book nothing and
// silently lose those shares.
func TestReconcileOpenOrders_DropsInconsistentFillProgress(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	mem := store.NewMemoryStorage()
	must.NoError(mem.Save(ctx, broker.Ledger{
		Version:  1,
		Lots:     []broker.LotState{},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
		Filled:   map[string]decimal.Decimal{"EX-BAD": decimal.NewFromInt(6)}, // observed 6
	}))

	// Contract-violating recovered order: Size() is the 4-unit remainder, not the
	// original 10 (built straight from the remainder, with no Partial applied).
	bad := buyLimit("AAA", 4, 100)
	bad.SetID("EX-BAD")
	bk, _ := newReportingBroker(t, 100_000, []order.Order{bad}, nil)
	bk.SetStorage(mem)

	must.NoError(bk.Restore(ctx))
	must.NoError(bk.ReconcileOpenOrders(ctx))

	bk.Listen(ctx, completedFill(bad, 100, 4))

	// The stale 6 (> Size 4) was dropped, so the completion books the 4-unit remainder
	// rather than a negative increment that books nothing.
	pos, _, ok := bk.StrategyPosition("", "AAA")
	must.True(ok, "the completion still books the remainder, not nothing")
	eqDec(t, 4, pos.Size, "the reported remainder books despite the inconsistent progress")
}

// TestReconcileOpenOrders_LegacyLedgerAvoidsDoubleCount verifies the migration path for a
// ledger written before per-order fill progress existed (schema v1, no Filled): the
// restored lots already include the observed partial, so recovery seeds from the
// exchange-reported filled quantity (not zero) to avoid booking that partial a second
// time on completion.
func TestReconcileOpenOrders_LegacyLedgerAvoidsDoubleCount(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	// A legacy (pre-Filled) ledger: version 1, lots include alpha's observed 6, no Filled.
	mem := store.NewMemoryStorage()
	must.NoError(mem.Save(ctx, broker.Ledger{
		Version: 1,
		Lots: []broker.LotState{{
			Strategy: "alpha", Item: &item.Item{Code: "AAA"},
			Size: decimal.NewFromInt(6), Cost: decimal.NewFromInt(600), Peak: decimal.NewFromInt(100),
		}},
		Realized: map[string]decimal.Decimal{},
		Fees:     map[string]decimal.Decimal{},
		// no Filled — the old schema did not persist per-order progress
	}))

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-LEG")
	buy.Partial(decimal.NewFromInt(6)) // exchange: 6/10 filled, 4 remaining
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)
	bk.SetStorage(mem)

	must.NoError(bk.Restore(ctx))
	must.NoError(bk.ReconcileOpenOrders(ctx))

	bk.Listen(ctx, completedFill(buy, 100, 4))

	// Legacy path seeds from the exchange's 6 filled, so the completion books only the 4
	// remainder — alpha's restored 6 is not double-counted: total 10 units, equity 101000.
	// (Treating it as unobserved would book 10 again → 16 units / 101600.)
	eqDec(t, 101_000, bk.Equity(), "legacy ledger: persisted partial not double-counted")
}

// TestReconcileOpenOrders_MarksRecoveredOrdersWorking verifies a recovered order's public
// status reflects that it is live on the exchange (Accepted), not the Created status of a
// freshly constructed order — so operator/strategy code filtering open orders by status
// does not skip a working order. A partially filled one keeps its Partial status.
func TestReconcileOpenOrders_MarksRecoveredOrdersWorking(t *testing.T) {
	must := require.New(t)

	resting := buyLimit("AAA", 10, 100) // Created, as freshly constructed
	resting.SetID("EX-R")
	partial := buyLimit("BBB", 10, 100)
	partial.SetID("EX-P")
	partial.Partial(decimal.NewFromInt(3)) // already Partial
	bk, _ := newReportingBroker(t, 100_000, []order.Order{resting, partial}, nil)

	must.NoError(bk.ReconcileOpenOrders(context.Background()))

	must.Equal(order.Accepted, bk.Orders("AAA")[0].Status(), "a resting recovered order reads as Accepted, not Created")
	must.Equal(order.Partial, bk.Orders("BBB")[0].Status(), "a partially filled recovered order keeps Partial")
}

// TestReconcileOpenOrders_NoReporterIsNoop verifies an adapter that does not report
// open orders (e.g. a backtest market) makes reconciliation an inert no-op.
func TestReconcileOpenOrders_NoReporterIsNoop(t *testing.T) {
	bk, _ := newBrokerUnderTest(t, 100_000, 0) // plain MockMarket: no OpenOrderReporter

	require.NoError(t, bk.ReconcileOpenOrders(context.Background()))
	eqDec(t, 100_000, bk.Available(), "a non-reporting adapter leaves state untouched")
}

// TestReconcileOpenOrders_PropagatesError verifies an OpenOrders failure surfaces, so
// Start fails fast rather than trading while blind to the exchange's working orders.
func TestReconcileOpenOrders_PropagatesError(t *testing.T) {
	bk, _ := newReportingBroker(t, 100_000, nil, errors.New("exchange down"))

	err := bk.ReconcileOpenOrders(context.Background())
	require.ErrorContains(t, err, "exchange down")
}

// TestReconcileOpenOrders_Idempotent verifies reconciling twice (a retried Start)
// replaces the open set rather than accumulating duplicate orders or reservations.
func TestReconcileOpenOrders_Idempotent(t *testing.T) {
	must := require.New(t)

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-3")
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)

	must.NoError(bk.ReconcileOpenOrders(context.Background()))
	must.NoError(bk.ReconcileOpenOrders(context.Background()))

	must.Len(bk.Orders("AAA"), 1, "reconciling twice does not double-count the order")
	eqDec(t, 99_000, bk.Available(), "the reservation is not applied twice")
}

// TestReconcileOpenOrders_SkipsTerminal verifies a defensively-reported terminal order
// is ignored: it would never receive a follow-up event to release a reservation.
func TestReconcileOpenOrders_SkipsTerminal(t *testing.T) {
	must := require.New(t)

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("EX-4")
	buy.Complete() // terminal — must not be seeded into the open set
	bk, _ := newReportingBroker(t, 100_000, []order.Order{buy}, nil)

	must.NoError(bk.ReconcileOpenOrders(context.Background()))
	must.Empty(bk.Orders("AAA"), "a terminal order is not recovered")
	eqDec(t, 100_000, bk.Available(), "a terminal order reserves nothing")
}
