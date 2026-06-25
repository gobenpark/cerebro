package indicator_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
)

var testBase = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// dec is a shorthand for an integer-valued decimal used across the indicator tests.
func dec(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

// eqDec asserts got equals the integer amount want, comparing numerically so a
// different decimal scale does not fail the check.
func eqDec(t *testing.T, want int64, got decimal.Decimal) {
	t.Helper()
	assert.Truef(t, decimal.NewFromInt(want).Equal(got), "want %d, got %s", want, got.String())
}

// closeCandles builds candles whose OHLC all equal the given close prices.
func closeCandles(closes ...int64) indicator.Candles {
	cds := make(indicator.Candles, len(closes))
	for i, c := range closes {
		cds[i] = &indicator.Candle{
			Open:  dec(c),
			High:  dec(c),
			Low:   dec(c),
			Close: dec(c),
			Date:  testBase.Add(time.Duration(i) * time.Hour),
		}
	}
	return cds
}

func TestCandles_Mean(t *testing.T) {
	is := assert.New(t)
	is.InDelta(20.0, closeCandles(10, 20, 30).Mean(), 1e-9)
}

func TestCandles_StandardDeviation(t *testing.T) {
	is := assert.New(t)
	// [10,20,30]: mean 20, sample variance (100+0+100)/(3-1)=100, sd=10.
	is.InDelta(10.0, closeCandles(10, 20, 30).StandardDeviation(), 1e-9)
}

func TestCandles_BollingerBand(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	cds := closeCandles(10, 20, 30)
	bottom, mid, top := cds.BollingerBand(2)
	must.Len(mid, 3)

	// Only the last index (i=2) is filled: window c[0:3] -> mean 20, sd 10.
	is.InDelta(20.0, mid[2].Data, 1e-9)
	is.InDelta(40.0, top[2].Data, 1e-9)   // mean + 2*sd, rounded
	is.InDelta(0.0, bottom[2].Data, 1e-9) // mean - 2*sd, rounded
	is.InDelta(0.0, mid[1].Data, 1e-9)    // i<period -> empty
}

// TestCandles_EnvelopeUsesDownRatioForLowerBand guards the fix where the lower
// band was computed with the up ratio instead of down.
func TestCandles_EnvelopeUsesDownRatioForLowerBand(t *testing.T) {
	is := assert.New(t)

	cds := closeCandles(100, 100, 100)
	sma, upper, lower := cds.Envelope(1, 0.1, 0.2) // up 10%, down 20%

	last := len(cds) - 1
	is.InDelta(100.0, sma[last].Data, 1e-9)
	is.InDelta(110.0, upper[last].Data, 1e-9) // 100 + 100*0.1
	is.InDelta(80.0, lower[last].Data, 1e-9, "lower band must use the down ratio (0.2), not up")
}

// TestCandles_StochasticFastAveragesDOverDPeriod guards the fix where %D was
// averaged over k instead of d.
func TestCandles_StochasticFastAveragesDOverDPeriod(t *testing.T) {
	is := assert.New(t)

	ohlc := func(high, low, cl int64) *indicator.Candle {
		return &indicator.Candle{High: dec(high), Low: dec(low), Close: dec(cl), Date: testBase}
	}
	cds := indicator.Candles{
		ohlc(20, 10, 15),
		ohlc(30, 10, 20),
		ohlc(25, 5, 10),
	}

	k, d := cds.StochasticFast(2, 1) // d=2, period=1

	// %K over the period-window: i=1 -> (20-10)/(30-10)*100=50, i=2 -> (10-5)/(30-5)*100=20.
	is.InDelta(50.0, k[1].Data, 1e-9)
	is.InDelta(20.0, k[2].Data, 1e-9)

	// %D averages %K over d=2: D[2] = mean(K[1], K[2]) = mean(50, 20) = 35.
	is.InDelta(35.0, d[2].Data, 1e-9, "%D must average over the d period")
}

func TestCandles_MACDReturnsEmptyWhenShorterThanSlow(t *testing.T) {
	is := assert.New(t)

	macd, signal := closeCandles(1, 2, 3).MACD(12, 26, 9) // len 3 < slow 26
	is.Empty(macd)
	is.Empty(signal)
}

// TestCandles_VWAP covers the anchored VWAP: it accumulates the typical-price
// (H+L+C)/3 weighted by volume from the series start, weights by volume (not a
// plain mean), uses the typical price rather than the close, and a zero-volume
// bar leaves the running value unchanged.
func TestCandles_VWAP(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	hlcv := func(h, l, cl, vol int64) *indicator.Candle {
		return &indicator.Candle{High: dec(h), Low: dec(l), Close: dec(cl), Volume: vol, Date: testBase}
	}
	cds := indicator.Candles{
		hlcv(40, 10, 10, 1), // typical (40+10+10)/3 = 20, vol 1 -> proves typical != close (10)
		hlcv(30, 30, 30, 3), // typical 30, vol 3
		hlcv(50, 50, 50, 0), // zero-volume bar must not move the VWAP
	}

	v := cds.VWAP()
	must.Len(v, 3)
	is.InDelta(20.0, v[0].Data, 1e-9)  // 20*1 / 1 = 20 (typical, not close)
	is.InDelta(27.5, v[1].Data, 1e-9)  // (20*1 + 30*3) / 4 = 110/4 = 27.5
	is.InDelta(27.5, v[2].Data, 1e-9)  // zero-volume bar adds nothing
	is.True(v[2].Date.Equal(testBase)) // result carries the candle's date
}

// TestCandles_VWAPZeroVolumeIsZero guards that a zero-volume prefix yields 0
// rather than a NaN from dividing by zero cumulative volume.
func TestCandles_VWAPZeroVolumeIsZero(t *testing.T) {
	is := assert.New(t)

	cds := indicator.Candles{
		{High: dec(10), Low: dec(10), Close: dec(10), Volume: 0, Date: testBase},
	}
	v := cds.VWAP()
	is.InDelta(0.0, v[0].Data, 1e-9)
}
