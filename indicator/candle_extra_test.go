package indicator

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCandle_Shape_Bull(t *testing.T) {
	nd := decimal.NewFromInt
	c := &Candle{Open: nd(100), Close: nd(110), High: nd(115), Low: nd(95)}

	assert.True(t, c.IsBull())
	assert.False(t, c.IsBear())
	assert.False(t, c.IsDoji())
	assert.True(t, c.Range().Equal(nd(20)))
	assert.True(t, c.Body().Equal(nd(10)))
	assert.True(t, c.UpperWick().Equal(nd(5))) // 115 - max(100,110)
	assert.True(t, c.LowerWick().Equal(nd(5))) // min(100,110) - 95
	assert.InDelta(t, 0.50, c.BodyRatio(), 1e-9)
	assert.InDelta(t, 0.25, c.UpperWickRatio(), 1e-9)
	assert.InDelta(t, 0.25, c.LowerWickRatio(), 1e-9)
}

func TestCandle_Shape_BearAndDoji(t *testing.T) {
	nd := decimal.NewFromInt

	bear := &Candle{Open: nd(110), Close: nd(100), High: nd(112), Low: nd(98)}
	assert.True(t, bear.IsBear())
	assert.False(t, bear.IsBull())
	assert.True(t, bear.UpperWick().Equal(nd(2))) // 112 - max(110,100)
	assert.True(t, bear.LowerWick().Equal(nd(2))) // min(110,100) - 98

	// A flat candle (high == low) must not divide by zero.
	doji := &Candle{Open: nd(100), Close: nd(100), High: nd(100), Low: nd(100)}
	assert.True(t, doji.IsDoji())
	assert.InDelta(t, 0.0, doji.BodyRatio(), 1e-9)
	assert.InDelta(t, 0.0, doji.UpperWickRatio(), 1e-9)
	assert.InDelta(t, 0.0, doji.LowerWickRatio(), 1e-9)
}

func TestCandle_NoUpperWick(t *testing.T) {
	nd := decimal.NewFromInt
	// A clean bull candle with no upper wick: high == close.
	c := &Candle{Open: nd(100), Close: nd(110), High: nd(110), Low: nd(100)}
	assert.True(t, c.UpperWick().IsZero())
	assert.InDelta(t, 0.0, c.UpperWickRatio(), 1e-9)
	assert.InDelta(t, 1.0, c.BodyRatio(), 1e-9)
}

func shapeHLCandles(highs, lows []int64) Candles {
	cs := make(Candles, len(highs))
	base := time.Unix(0, 0)
	for i := range highs {
		cs[i] = &Candle{
			Date: base.Add(time.Duration(i) * time.Minute),
			High: decimal.NewFromInt(highs[i]),
			Low:  decimal.NewFromInt(lows[i]),
		}
	}
	return cs
}

func TestCandles_HighestLowest(t *testing.T) {
	nd := decimal.NewFromInt
	cs := shapeHLCandles([]int64{10, 20, 15}, []int64{5, 12, 8})

	assert.True(t, cs.Highest(2).Equal(nd(20)))  // last two highs: 20, 15
	assert.True(t, cs.Highest(3).Equal(nd(20)))  // all
	assert.True(t, cs.Highest(10).Equal(nd(20))) // clamps to all
	assert.True(t, cs.Lowest(2).Equal(nd(8)))    // last two lows: 12, 8
	assert.True(t, cs.Lowest(10).Equal(nd(5)))   // clamps to all

	// period <= 0 means the whole series; a pathological negative must not
	// overflow the start index or panic.
	assert.True(t, cs.Highest(0).Equal(nd(20)))
	assert.True(t, cs.Highest(-100).Equal(nd(20)))
	assert.True(t, cs.Lowest(0).Equal(nd(5)))
	assert.True(t, cs.Lowest(-100).Equal(nd(5)))

	var empty Candles
	assert.True(t, empty.Highest(5).IsZero())
	assert.True(t, empty.Lowest(5).IsZero())
	assert.True(t, empty.Highest(-1).IsZero()) // empty + non-positive period
	assert.True(t, empty.Lowest(0).IsZero())
}

func shapeCloseCandles(closes ...int64) Candles {
	cs := make(Candles, len(closes))
	base := time.Unix(0, 0)
	for i, v := range closes {
		cs[i] = &Candle{
			Date:  base.Add(time.Duration(i) * time.Minute),
			Close: decimal.NewFromInt(v),
		}
	}
	return cs
}

func TestCandles_SMA(t *testing.T) {
	cs := shapeCloseCandles(10, 20, 30, 40)

	sma := cs.SMA(2)
	assert.Len(t, sma, 4)
	assert.InDelta(t, 0.0, sma[0].Data, 1e-9) // warmup, before period-1
	assert.InDelta(t, 15.0, sma[1].Data, 1e-9)
	assert.InDelta(t, 25.0, sma[2].Data, 1e-9)
	assert.InDelta(t, 35.0, sma[3].Data, 1e-9)

	sma1 := cs.SMA(1)
	assert.InDelta(t, 10.0, sma1[0].Data, 1e-9)
	assert.InDelta(t, 40.0, sma1[3].Data, 1e-9)

	// period <= 0 normalizes to 1 (each close is its own average).
	sma0 := cs.SMA(0)
	assert.InDelta(t, 10.0, sma0[0].Data, 1e-9)
	assert.InDelta(t, 40.0, sma0[3].Data, 1e-9)
	smaNeg := cs.SMA(-5)
	assert.InDelta(t, 40.0, smaNeg[3].Data, 1e-9)

	// Empty input yields an empty result (no panic, aligns index-for-index),
	// including the non-positive normalize-to-1 path.
	var empty Candles
	assert.Empty(t, empty.SMA(3))
	assert.Empty(t, empty.SMA(0))
	assert.Empty(t, empty.SMA(-1))
}
