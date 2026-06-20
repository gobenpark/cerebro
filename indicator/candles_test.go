package indicator_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
)

var testBase = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// closeCandles builds candles whose OHLC all equal the given close prices.
func closeCandles(closes ...int64) indicator.Candles {
	cds := make(indicator.Candles, len(closes))
	for i, c := range closes {
		cds[i] = &indicator.Candle{
			Open:  c,
			High:  c,
			Low:   c,
			Close: c,
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
		return &indicator.Candle{High: high, Low: low, Close: cl, Date: testBase}
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
