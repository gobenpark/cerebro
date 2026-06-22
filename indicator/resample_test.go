package indicator_test

import (
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

// TestResampler_NewBucketSeededFromTick locks the fix for the bug where crossing
// a bucket boundary appended an empty candle (all-zero OHLC) instead of seeding
// the new bucket from the incoming tick.
func TestResampler_NewBucketSeededFromTick(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	base := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	compress := time.Minute

	// Empty history: first bucket is created from the tick.
	cds := indicator.Resampler(indicator.Candles{}, indicator.Tick{Date: base, Code: "AAA", Price: dec(100), Volume: 5}, compress)
	must.Len(cds, 1)
	eqDec(t, 100, cds[0].Open)
	eqDec(t, 100, cds[0].Close)

	// Same bucket: the open candle is updated in place.
	cds = indicator.Resampler(cds, indicator.Tick{Date: base.Add(20 * time.Second), Code: "AAA", Price: dec(120), Volume: 2}, compress)
	must.Len(cds, 1)
	eqDec(t, 120, cds[0].High)
	eqDec(t, 100, cds[0].Low)
	eqDec(t, 120, cds[0].Close)
	is.Equal(int64(7), cds[0].Volume)

	// Crossing the boundary: the new bucket must carry the tick's values, not zeros.
	cds = indicator.Resampler(cds, indicator.Tick{Date: base.Add(2 * compress), Code: "AAA", Price: dec(130), Volume: 4}, compress)
	must.Len(cds, 2)
	eqDec(t, 130, cds[1].Open)
	eqDec(t, 130, cds[1].High)
	eqDec(t, 130, cds[1].Low)
	eqDec(t, 130, cds[1].Close)
	is.Equal(int64(4), cds[1].Volume)
	is.Equal("AAA", cds[1].Code)
	is.Equal(base.Add(2*compress), cds[1].Date)
}
