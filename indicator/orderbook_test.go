package indicator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
)

// book builds an OrderBook from best-first (price,size) pairs for each side.
func book(bids, asks [][2]int64) indicator.OrderBook {
	conv := func(rows [][2]int64) []indicator.Level {
		ls := make([]indicator.Level, len(rows))
		for i, r := range rows {
			ls[i] = indicator.Level{Price: dec(r[0]), Size: dec(r[1])}
		}
		return ls
	}
	return indicator.OrderBook{Code: "AAA", Bids: conv(bids), Asks: conv(asks)}
}

func TestOrderBook_BestBidAsk(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	ob := book([][2]int64{{99, 5}, {98, 3}}, [][2]int64{{101, 4}, {102, 7}})

	bid, ok := ob.BestBid()
	must.True(ok)
	eqDec(t, 99, bid.Price)
	eqDec(t, 5, bid.Size)

	ask, ok := ob.BestAsk()
	must.True(ok)
	eqDec(t, 101, ask.Price)

	// Empty sides report ok=false rather than a zero-value level.
	_, ok = (indicator.OrderBook{}).BestBid()
	is.False(ok)
	_, ok = (indicator.OrderBook{}).BestAsk()
	is.False(ok)
}

func TestOrderBook_SpreadAndMid(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	ob := book([][2]int64{{99, 5}}, [][2]int64{{101, 4}})

	sp, ok := ob.Spread()
	must.True(ok)
	eqDec(t, 2, sp) // 101 - 99

	mid, ok := ob.Mid()
	must.True(ok)
	eqDec(t, 100, mid) // (99 + 101) / 2

	// A one-sided book leaves spread and mid undefined.
	oneSided := book([][2]int64{{99, 5}}, nil)
	_, ok = oneSided.Spread()
	is.False(ok)
	_, ok = oneSided.Mid()
	is.False(ok)
}

func TestOrderBook_Imbalance(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	// Top level: bid 6 vs ask 2 -> (6-2)/(6+2) = 0.5.
	ob := book([][2]int64{{99, 6}, {98, 10}}, [][2]int64{{101, 2}, {102, 10}})
	imb, ok := ob.Imbalance(1)
	must.True(ok)
	eqDec(t, 0, imb.Sub(dec(1).Div(dec(2)))) // 0.5

	// Full depth: bids 16 vs asks 12 -> 4/28.
	imbAll, ok := ob.Imbalance(0)
	must.True(ok)
	is.True(imbAll.GreaterThan(dec(0)), "more resting bids than asks is positive imbalance")

	// Only bids -> +1; only asks -> -1.
	onlyBids := book([][2]int64{{99, 5}}, nil)
	imbB, ok := onlyBids.Imbalance(0)
	must.True(ok)
	eqDec(t, 1, imbB)

	onlyAsks := book(nil, [][2]int64{{101, 5}})
	imbA, ok := onlyAsks.Imbalance(0)
	must.True(ok)
	eqDec(t, -1, imbA)

	// Empty book: undefined.
	_, ok = (indicator.OrderBook{}).Imbalance(0)
	is.False(ok)
}
