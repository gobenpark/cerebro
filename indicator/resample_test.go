package indicator_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
)

func TestResample_GroupsTicksIntoBuckets(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	// Intentionally unsorted to also exercise the internal sort.
	ticks := []indicator.Tick{
		{Date: base, Code: "AAA", Price: dec(100), Volume: 5},
		{Date: base.Add(30 * time.Second), Code: "AAA", Price: dec(110), Volume: 3},
		{Date: base.Add(10 * time.Second), Code: "AAA", Price: dec(95), Volume: 1},
		{Date: base.Add(90 * time.Second), Code: "AAA", Price: dec(90), Volume: 2},
	}

	cds := indicator.Resample(ticks, time.Minute)
	must.Len(cds, 2)

	// Bucket [09:00, 09:01): ticks 100, 95, 110 in time order.
	is.Equal(base, cds[0].Date)
	eqDec(t, 100, cds[0].Open)
	eqDec(t, 110, cds[0].High)
	eqDec(t, 95, cds[0].Low)
	eqDec(t, 110, cds[0].Close)
	is.Equal(int64(9), cds[0].Volume)

	// Bucket [09:01, 09:02): single tick 90.
	is.Equal(base.Add(time.Minute), cds[1].Date)
	eqDec(t, 90, cds[1].Open)
	eqDec(t, 90, cds[1].Close)
	is.Equal(int64(2), cds[1].Volume)
}

// TestResampler_FormsAndClosesBars walks a Resampler through opening a bar,
// folding a same-bucket tick, and closing on a boundary crossing, checking the
// close signal, the closed bar's values, and the freshly seeded forming bar.
func TestResampler_FormsAndClosesBars(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	compress := time.Minute
	r := indicator.NewResampler(compress)

	// First tick opens the bar; nothing closes yet.
	_, ok := r.Add(indicator.Tick{Date: base, Code: "AAA", Price: dec(100), Volume: 5})
	is.False(ok)
	cur, has := r.Current()
	must.True(has)
	eqDec(t, 100, cur.Open)
	eqDec(t, 100, cur.Close)
	is.Empty(r.History())

	// Same bucket: the forming bar updates in place, still no close.
	_, ok = r.Add(indicator.Tick{Date: base.Add(20 * time.Second), Code: "AAA", Price: dec(120), Volume: 2})
	is.False(ok)
	cur, _ = r.Current()
	eqDec(t, 120, cur.High)
	eqDec(t, 100, cur.Low)
	eqDec(t, 120, cur.Close)
	is.Equal(int64(7), cur.Volume)

	// Crossing the boundary closes the forming bar and returns it.
	closed, ok := r.Add(indicator.Tick{Date: base.Add(2 * compress), Code: "AAA", Price: dec(130), Volume: 4})
	must.True(ok, "crossing a bucket boundary closes the forming bar")
	eqDec(t, 100, closed.Open)
	eqDec(t, 120, closed.High)
	eqDec(t, 100, closed.Low)
	eqDec(t, 120, closed.Close)
	is.Equal(int64(7), closed.Volume)
	is.Equal(base, closed.Date)

	// The new forming bar carries the boundary tick's values, not zeros.
	cur, _ = r.Current()
	eqDec(t, 130, cur.Open)
	eqDec(t, 130, cur.High)
	eqDec(t, 130, cur.Low)
	is.Equal(int64(4), cur.Volume)
	is.Equal("AAA", cur.Code)
	is.Equal(base.Add(2*compress), cur.Date)

	// History holds exactly the one closed bar.
	must.Len(r.History(), 1)
	eqDec(t, 120, r.History()[0].Close)
}

// eqCandle asserts two candles are equal across every OHLCV field.
func eqCandle(t *testing.T, want, got *indicator.Candle) {
	t.Helper()
	is := assert.New(t)
	is.Equal(want.Date, got.Date, "date")
	is.Equal(want.Code, got.Code, "code")
	is.Truef(want.Open.Equal(got.Open), "open: want %s got %s", want.Open, got.Open)
	is.Truef(want.High.Equal(got.High), "high: want %s got %s", want.High, got.High)
	is.Truef(want.Low.Equal(got.Low), "low: want %s got %s", want.Low, got.Low)
	is.Truef(want.Close.Equal(got.Close), "close: want %s got %s", want.Close, got.Close)
	is.Equal(want.Volume, got.Volume, "volume")
}

// TestResample_ExactBoundaryTicksEachStartNewBucket locks the boundary rule for
// the batch path: a tick landing exactly on a bucket edge (common for ticks
// stamped at whole minutes) opens a new bucket rather than folding into the prior.
func TestResample_ExactBoundaryTicksEachStartNewBucket(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	ticks := []indicator.Tick{
		{Date: base, Code: "AAA", Price: dec(100), Volume: 1},
		{Date: base.Add(time.Minute), Code: "AAA", Price: dec(101), Volume: 1},
		{Date: base.Add(2 * time.Minute), Code: "AAA", Price: dec(102), Volume: 1},
	}

	cds := indicator.Resample(ticks, time.Minute)

	must.Len(cds, 3, "three whole-minute ticks must produce three buckets")
	is.Equal(base, cds[0].Date)
	eqDec(t, 100, cds[0].Close)
	is.Equal(base.Add(time.Minute), cds[1].Date)
	eqDec(t, 101, cds[1].Close)
	is.Equal(base.Add(2*time.Minute), cds[2].Date)
	eqDec(t, 102, cds[2].Close)
}

// TestResampler_ExactBoundaryTickStartsNewBucket locks the same boundary rule for
// the incremental path — the case that previously folded a boundary tick into the
// prior candle and dropped its bucket.
func TestResampler_ExactBoundaryTickStartsNewBucket(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	compress := time.Minute
	r := indicator.NewResampler(compress)

	_, ok := r.Add(indicator.Tick{Date: base, Code: "AAA", Price: dec(100), Volume: 5})
	is.False(ok)

	// A tick exactly on the boundary closes the first bar instead of folding into it.
	closed, ok := r.Add(indicator.Tick{Date: base.Add(compress), Code: "AAA", Price: dec(101), Volume: 3})
	must.True(ok, "a tick exactly on the boundary opens a new bucket")
	eqDec(t, 100, closed.Close)
	is.Equal(int64(5), closed.Volume, "first bucket is untouched by the boundary tick")

	cur, _ := r.Current()
	is.Equal(base.Add(compress), cur.Date)
	eqDec(t, 101, cur.Open)
	is.Equal(int64(3), cur.Volume)
}

// TestResample_GapLeavesNoEmptyBuckets verifies a time gap between ticks produces
// no empty (zero-OHLC) candles — only buckets that actually contain a tick exist.
func TestResample_GapLeavesNoEmptyBuckets(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	ticks := []indicator.Tick{
		{Date: base, Code: "AAA", Price: dec(100), Volume: 1},
		{Date: base.Add(5 * time.Minute), Code: "AAA", Price: dec(120), Volume: 2},
	}

	cds := indicator.Resample(ticks, time.Minute)

	must.Len(cds, 2, "the 4-minute gap must not synthesize empty candles")
	is.Equal(base, cds[0].Date)
	is.Equal(base.Add(5*time.Minute), cds[1].Date)
	eqDec(t, 120, cds[1].Open)
}

// TestResample_DoesNotMutateInput verifies the caller's slice is left in its
// original order (Resample sorts an internal copy).
func TestResample_DoesNotMutateInput(t *testing.T) {
	is := assert.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	ticks := []indicator.Tick{
		{Date: base.Add(30 * time.Second), Code: "AAA", Price: dec(110), Volume: 1},
		{Date: base, Code: "AAA", Price: dec(100), Volume: 1},
		{Date: base.Add(10 * time.Second), Code: "AAA", Price: dec(95), Volume: 1},
	}
	want := make([]time.Time, len(ticks))
	for i := range ticks {
		want[i] = ticks[i].Date
	}

	_ = indicator.Resample(ticks, time.Minute)

	for i := range ticks {
		is.Equal(want[i], ticks[i].Date, "Resample must not reorder the caller's slice")
	}
}

func TestResample_EmptyInput(t *testing.T) {
	assert.Empty(t, indicator.Resample(nil, time.Minute))
}

// TestResampler_MatchesResample is the unification guard: feeding a tick series
// incrementally through Resampler must yield exactly the candles the batch
// Resample produces, so the two paths can never drift on bucketing or folding.
func TestResampler_MatchesResample(t *testing.T) {
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	compress := time.Minute
	// Time-ordered series exercising same-bucket folds, an exact boundary, and a gap.
	ticks := []indicator.Tick{
		{Date: base, Code: "AAA", Price: dec(100), Volume: 5},
		{Date: base.Add(30 * time.Second), Code: "AAA", Price: dec(110), Volume: 1},
		{Date: base.Add(time.Minute), Code: "AAA", Price: dec(105), Volume: 2}, // exact boundary
		{Date: base.Add(90 * time.Second), Code: "AAA", Price: dec(95), Volume: 3},
		{Date: base.Add(5 * time.Minute), Code: "AAA", Price: dec(120), Volume: 4}, // gap
	}

	batch := indicator.Resample(ticks, compress)

	// Incremental "all bars" = the closed history plus the still-forming bar.
	r := indicator.NewResampler(compress)
	for _, tk := range ticks {
		r.Add(tk)
	}
	live := slices.Clone(r.History())
	if cur, ok := r.Current(); ok {
		live = append(live, &cur)
	}

	must.Len(batch, 3)
	must.Len(live, len(batch), "both paths must produce the same candle count")
	for i := range batch {
		eqCandle(t, batch[i], live[i])
	}

	// Pin the expected bucket contents so the cross-check cannot pass on two
	// equal-but-wrong results.
	eqDec(t, 100, batch[0].Open)
	eqDec(t, 110, batch[0].High)
	eqDec(t, 100, batch[0].Low)
	eqDec(t, 110, batch[0].Close)
	must.Equal(int64(6), batch[0].Volume)

	eqDec(t, 105, batch[1].Open)
	eqDec(t, 105, batch[1].High)
	eqDec(t, 95, batch[1].Low)
	eqDec(t, 95, batch[1].Close)
	must.Equal(int64(5), batch[1].Volume)

	eqDec(t, 120, batch[2].Open)
	must.Equal(int64(4), batch[2].Volume)
}

// TestResampler_WindowCapsHistory verifies WithWindow bounds the retained closed
// bars, keeping the most recent ones, so a long-running strategy does not grow
// memory without bound.
func TestResampler_WindowCapsHistory(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	compress := time.Minute
	r := indicator.NewResampler(compress, indicator.WithWindow(2))

	// One tick per minute across five buckets closes four bars; the fifth forms.
	for i := range 5 {
		r.Add(indicator.Tick{
			Date:   base.Add(time.Duration(i) * compress),
			Code:   "AAA",
			Price:  dec(int64(100 + i)),
			Volume: 1,
		})
	}

	h := r.History()
	must.Len(h, 2, "window caps retained closed bars to the most recent two")
	eqDec(t, 102, h[0].Close) // third bucket
	eqDec(t, 103, h[1].Close) // fourth bucket

	cur, ok := r.Current()
	must.True(ok)
	eqDec(t, 104, cur.Close) // fifth bucket still forming
	is.Equal(base.Add(4*compress), cur.Date)
}
