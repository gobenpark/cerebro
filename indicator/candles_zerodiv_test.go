package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// These guard the indicators against silent +Inf / NaN from dividing by zero —
// an empty/flat window must yield a defined value, never poison downstream math.

func TestCandles_MeanStdDev_DegenerateLength(t *testing.T) {
	var empty Candles
	assert.InDelta(t, 0.0, empty.Mean(), 1e-9)              // no samples: 0, not 0/0 = NaN
	assert.InDelta(t, 0.0, empty.StandardDeviation(), 1e-9) // n-1 denominator would be -1

	one := shapeCloseCandles(100)
	assert.InDelta(t, 100.0, one.Mean(), 1e-9)
	sd := one.StandardDeviation() // single sample: n-1 = 0 would divide by zero
	assert.False(t, math.IsNaN(sd))
	assert.InDelta(t, 0.0, sd, 1e-9)
}

func TestCandles_BollingerBand_NonPositivePeriod(t *testing.T) {
	// 25 ascending closes; period 0 must fall back to 20 rather than collapse the
	// window to one point (which makes the standard deviation undefined).
	closes := make([]int64, 25)
	for i := range closes {
		closes[i] = int64(100 + i)
	}
	cs := shapeCloseCandles(closes...)

	bottom, mid, top := cs.BollingerBand(0)
	assert.Len(t, mid, 25)

	last := len(cs) - 1 // last entry sees a full 20+1 window
	for _, v := range []float64{bottom[last].Data, mid[last].Data, top[last].Data} {
		assert.False(t, math.IsNaN(v))
		assert.False(t, math.IsInf(v, 0))
	}
	assert.GreaterOrEqual(t, top[last].Data, mid[last].Data)
	assert.GreaterOrEqual(t, mid[last].Data, bottom[last].Data)
}

func volumeCandles(closes, vols []int64) Candles {
	cs := make(Candles, len(closes))
	base := time.Unix(0, 0)
	for i := range closes {
		cs[i] = &Candle{
			Date:   base.Add(time.Duration(i) * time.Minute),
			Close:  decimal.NewFromInt(closes[i]),
			Volume: vols[i],
		}
	}
	return cs
}

func TestCandles_VolumeRatio_AllUp_NoInf(t *testing.T) {
	// Strictly ascending closes => no down-volume => zero denominator. Must report
	// 0, not +Inf.
	cs := volumeCandles([]int64{10, 20, 30, 40, 50}, []int64{100, 100, 100, 100, 100})
	for _, v := range cs.VolumeRatio(2) {
		assert.False(t, math.IsInf(v.Data, 0))
		assert.False(t, math.IsNaN(v.Data))
	}
}

func flatOHLCCandles(n int, price int64) Candles {
	cs := make(Candles, n)
	base := time.Unix(0, 0)
	p := decimal.NewFromInt(price)
	for i := range cs {
		cs[i] = &Candle{
			Date:  base.Add(time.Duration(i) * time.Minute),
			Open:  p,
			High:  p,
			Low:   p,
			Close: p,
		}
	}
	return cs
}

func TestCandles_StochasticFast_FlatWindow_Neutral(t *testing.T) {
	// A flat window (high == low across the lookback) has no range, so %K is
	// undefined; it must resolve to the neutral 50 instead of 0/0 = NaN.
	cs := flatOHLCCandles(20, 100)
	k, d := cs.StochasticFast(3, 5)
	for _, v := range k {
		assert.False(t, math.IsNaN(v.Data))
	}
	for _, v := range d {
		assert.False(t, math.IsNaN(v.Data))
	}
	assert.InDelta(t, 50.0, k[len(k)-1].Data, 1e-9) // last entry is in the flat region
}
