package broker_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
)

// fakeStorage is an in-memory broker.Storage that captures the last saved ledger
// and serves it back on Load, so a test can drive the persist path on one broker
// and the restore path on another.
type fakeStorage struct {
	saved  broker.Ledger
	hasOne bool
	saves  int
}

func (f *fakeStorage) Save(_ context.Context, l broker.Ledger) error {
	f.saved = l
	f.hasOne = true
	f.saves++
	return nil
}

func (f *fakeStorage) Load(_ context.Context) (broker.Ledger, error) {
	if !f.hasOne {
		return broker.Ledger{}, nil
	}
	return f.saved, nil
}

// TestStorage_PersistAndRestoreLedger drives fills through one broker, then
// restores the persisted ledger into a fresh broker and verifies the per-strategy
// realized PnL, fees, and open lots (size, average entry, high-water peak) all
// carry over.
func TestStorage_PersistAndRestoreLedger(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	ctx := context.Background()

	store := &fakeStorage{}
	src, mk := newBrokerUnderTest(t, 1_000_000, 1) // 1% commission
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	src.SetStorage(store)

	// alpha scales in (avg 150, peak 200) then trims 5 (realized (180-150)*5 = 150).
	low := buyLimit("AAA", 10, 100)
	must.NoError(src.Scoped("alpha").Order(ctx, low, false))
	src.Listen(ctx, completedFill(low, 100, 10))

	high := buyLimit("AAA", 10, 200)
	must.NoError(src.Scoped("alpha").Order(ctx, high, false))
	src.Listen(ctx, completedFill(high, 200, 10))

	trim := sellLimit("AAA", 5, 180)
	must.NoError(src.Scoped("alpha").Order(ctx, trim, false))
	src.Listen(ctx, completedFill(trim, 180, 5))

	// beta opens and fully closes (realized (130-100)*10 = 300, position flat).
	bopen := buyLimit("BBB", 10, 100)
	must.NoError(src.Scoped("beta").Order(ctx, bopen, false))
	src.Listen(ctx, completedFill(bopen, 100, 10))

	bclose := sellLimit("BBB", 10, 130)
	must.NoError(src.Scoped("beta").Order(ctx, bclose, false))
	src.Listen(ctx, completedFill(bclose, 130, 10))

	must.True(store.hasOne, "fills must have persisted a ledger")

	// A fresh broker restores from the same store.
	dst, _ := newBrokerUnderTest(t, 1_000_000, 1)
	dst.SetStorage(store)
	must.NoError(dst.Restore(ctx))

	eqDec(t, 150, dst.RealizedPnL("alpha"), "(180-150)*5 carried over")
	eqDec(t, 300, dst.RealizedPnL("beta"), "(130-100)*10 carried over")
	eqDec(t, 450, dst.TotalRealizedPnL())

	pos, peak, ok := dst.StrategyPosition("alpha", "AAA")
	must.True(ok, "alpha's open lot must be restored")
	eqDec(t, 15, pos.Size, "20 bought - 5 trimmed")
	eqDec(t, 150, pos.Price, "average entry survives restore")
	eqDec(t, 200, peak, "high-water fill price survives restore")

	_, _, ok = dst.StrategyPosition("beta", "BBB")
	is.False(ok, "beta is flat, so no lot should be restored")

	rep := dst.Report()
	must.Len(rep, 2)
	is.Equal("alpha", rep[0].Strategy)
	eqDec(t, 150, rep[0].Realized)
	is.True(decimal.NewFromInt(39).Equal(rep[0].Fees), "buy 10 + buy 20 + sell 9")
	is.Equal("beta", rep[1].Strategy)
	eqDec(t, 300, rep[1].Realized)
	is.True(decimal.NewFromInt(23).Equal(rep[1].Fees), "buy 10 + sell 13")
}

// TestStorage_RestoreFreshStartIsNoop verifies an empty store leaves the broker
// untouched, and that a broker with no store restores cleanly too.
func TestStorage_RestoreFreshStartIsNoop(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)
	ctx := context.Background()

	withStore, _ := newBrokerUnderTest(t, 100_000, 0)
	withStore.SetStorage(&fakeStorage{}) // never saved
	must.NoError(withStore.Restore(ctx))
	is.Empty(withStore.Report(), "nothing persisted means nothing to restore")

	noStore, _ := newBrokerUnderTest(t, 100_000, 0)
	must.NoError(noStore.Restore(ctx), "restore without a store is a no-op")
	is.Empty(noStore.Report())
}

// TestStorage_PersistsOnlyOnFills guards that order transitions which do not move
// the ledger (e.g. acceptance) trigger no write, so only booked fills persist.
func TestStorage_PersistsOnlyOnFills(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	store := &fakeStorage{}
	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	bk.SetStorage(store)

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))

	bk.Listen(ctx, market.ChangeOrderEvent{ID: buy.ID(), Action: order.Accepted})
	require.Zero(t, store.saves, "acceptance is not a fill and must not persist")

	bk.Listen(ctx, completedFill(buy, 100, 10))
	require.Equal(t, 1, store.saves, "the fill persists exactly once")
}
