package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/store"
)

// sampleLedger is a ledger with one open lot and per-strategy realized/fees, used
// to exercise serialization fidelity (decimal scale, item code) end to end.
func sampleLedger() broker.Ledger {
	return broker.Ledger{
		Version: 1,
		Lots: []broker.LotState{
			{
				Strategy: "alpha",
				Item:     &item.Item{Code: "AAA", Name: "Alpha Co"},
				Size:     decimal.NewFromInt(15),
				Cost:     decimal.NewFromInt(2250),
				Peak:     decimal.NewFromInt(200),
			},
		},
		Realized: map[string]decimal.Decimal{"alpha": decimal.NewFromFloat(150.5), "beta": decimal.NewFromInt(300)},
		Fees:     map[string]decimal.Decimal{"alpha": decimal.NewFromFloat(39.9)},
	}
}

// assertLedgerEqual compares two ledgers numerically so a benign decimal-scale
// difference from JSON round-tripping does not fail the check.
func assertLedgerEqual(t *testing.T, want, got broker.Ledger) {
	t.Helper()
	is := assert.New(t)
	is.Equal(want.Version, got.Version)

	require.Len(t, got.Lots, len(want.Lots))
	for i := range want.Lots {
		is.Equal(want.Lots[i].Strategy, got.Lots[i].Strategy)
		require.NotNil(t, got.Lots[i].Item)
		is.Equal(want.Lots[i].Item.Code, got.Lots[i].Item.Code)
		is.True(want.Lots[i].Size.Equal(got.Lots[i].Size), "size")
		is.True(want.Lots[i].Cost.Equal(got.Lots[i].Cost), "cost")
		is.True(want.Lots[i].Peak.Equal(got.Lots[i].Peak), "peak")
	}

	require.Len(t, got.Realized, len(want.Realized))
	for k, v := range want.Realized {
		is.Truef(v.Equal(got.Realized[k]), "realized[%s]", k)
	}
	require.Len(t, got.Fees, len(want.Fees))
	for k, v := range want.Fees {
		is.Truef(v.Equal(got.Fees[k]), "fees[%s]", k)
	}
}

func TestFileStorage_SaveLoadRoundTrip(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "ledger.json")
	fs := store.NewFileStorage(path)

	want := sampleLedger()
	must.NoError(fs.Save(ctx, want))

	got, err := fs.Load(ctx)
	must.NoError(err)
	assertLedgerEqual(t, want, got)
}

func TestFileStorage_LoadMissingFileIsFreshStart(t *testing.T) {
	must := require.New(t)

	fs := store.NewFileStorage(filepath.Join(t.TempDir(), "does-not-exist.json"))

	l, err := fs.Load(context.Background())
	must.NoError(err, "a missing file is a fresh start, not an error")
	must.Zero(l.Version, "a fresh start reports Version 0")
}

// TestFileStorage_SaveOverwritesAtomically verifies a second Save replaces the
// first and leaves no stray temp files in the directory.
func TestFileStorage_SaveOverwritesAtomically(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.json")
	fs := store.NewFileStorage(path)

	must.NoError(fs.Save(ctx, sampleLedger()))

	updated := sampleLedger()
	updated.Realized["alpha"] = decimal.NewFromInt(999)
	must.NoError(fs.Save(ctx, updated))

	got, err := fs.Load(ctx)
	must.NoError(err)
	must.True(decimal.NewFromInt(999).Equal(got.Realized["alpha"]), "second save must win")

	entries, err := os.ReadDir(dir)
	must.NoError(err)
	must.Len(entries, 1, "the atomic write must leave only the ledger file, no temp leftovers")
}

func TestMemoryStorage_RoundTrip(t *testing.T) {
	must := require.New(t)
	ctx := context.Background()

	ms := store.NewMemoryStorage()

	l, err := ms.Load(ctx)
	must.NoError(err)
	must.Zero(l.Version, "an empty memory store is a fresh start")

	want := sampleLedger()
	must.NoError(ms.Save(ctx, want))

	got, err := ms.Load(ctx)
	must.NoError(err)
	assertLedgerEqual(t, want, got)
}
