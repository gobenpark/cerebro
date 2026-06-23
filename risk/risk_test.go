package risk_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/risk"
)

func dec(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

func buy(code string, size, price int64) order.Order {
	return order.NewOrder(&item.Item{Code: code}, order.Buy, order.Limit, dec(size), dec(price))
}

func sell(code string, size, price int64) order.Order {
	return order.NewOrder(&item.Item{Code: code}, order.Sell, order.Limit, dec(size), dec(price))
}

func held(code string, size, price int64) position.Position {
	return position.Position{Item: &item.Item{Code: code}, Size: dec(size), Price: dec(price)}
}

func TestMaxOrderValue(t *testing.T) {
	is := assert.New(t)
	r := risk.MaxOrderValue(dec(500))
	is.NoError(r.Check(buy("AAA", 4, 100), risk.Snapshot{}))                       // 400 <= 500
	is.ErrorIs(r.Check(buy("AAA", 10, 100), risk.Snapshot{}), risk.ErrOrderTooBig) // 1000 > 500
}

func TestMaxPositionPct(t *testing.T) {
	is := assert.New(t)
	r := risk.MaxPositionPct(0.2) // 20% of balance
	s := risk.Snapshot{Balance: dec(100_000)}

	is.NoError(r.Check(buy("AAA", 100, 100), s))                      // 10_000 <= 20_000
	is.ErrorIs(r.Check(buy("AAA", 300, 100), s), risk.ErrPositionCap) // 30_000 > 20_000
	is.NoError(r.Check(sell("AAA", 1000, 100), s), "sells are not capped")

	// An existing position counts toward exposure.
	s.Positions = []position.Position{held("AAA", 150, 100)}          // 15_000 held
	is.ErrorIs(r.Check(buy("AAA", 100, 100), s), risk.ErrPositionCap) // 15_000 + 10_000 > 20_000
}

func TestMaxLoss(t *testing.T) {
	is := assert.New(t)
	r := risk.MaxLoss(dec(1000)) // stop opening once the loss reaches 1000

	is.NoError(r.Check(buy("AAA", 1, 1), risk.Snapshot{Realized: dec(-500)}), "within the loss budget")
	is.ErrorIs(r.Check(buy("AAA", 1, 1), risk.Snapshot{Realized: dec(-1000)}), risk.ErrLossLimit, "loss limit reached")
	is.NoError(r.Check(sell("AAA", 1, 1), risk.Snapshot{Realized: dec(-2000)}), "sells (exits) are never blocked")
}

func TestMaxOpenPositions(t *testing.T) {
	is := assert.New(t)
	r := risk.MaxOpenPositions(2)
	s := risk.Snapshot{Positions: []position.Position{held("AAA", 1, 1), held("BBB", 1, 1)}}

	is.ErrorIs(r.Check(buy("CCC", 1, 1), s), risk.ErrTooManyOpen, "third distinct code rejected")
	is.NoError(r.Check(buy("AAA", 1, 1), s), "adding to an existing position is allowed")
}

func TestMaxPositionPct_CountsPendingBuys(t *testing.T) {
	is := assert.New(t)
	r := risk.MaxPositionPct(0.2) // cap 20_000 of 100_000
	s := risk.Snapshot{
		Balance: dec(100_000),
		Open:    []order.Order{buy("AAA", 150, 100)}, // 15_000 pending for AAA
	}
	// 15_000 pending + 10_000 new for the same code exceeds the 20_000 cap.
	is.ErrorIs(r.Check(buy("AAA", 100, 100), s), risk.ErrPositionCap)
}

func TestMaxOpenPositions_CountsPendingNewCodes(t *testing.T) {
	is := assert.New(t)
	r := risk.MaxOpenPositions(2)
	s := risk.Snapshot{
		Positions: []position.Position{held("AAA", 1, 1)}, // 1 held
		Open:      []order.Order{buy("BBB", 1, 1)},        // 1 pending new code
	}
	// AAA(held) + BBB(pending) + CCC(new) = 3 distinct codes > cap 2.
	is.ErrorIs(r.Check(buy("CCC", 1, 1), s), risk.ErrTooManyOpen)
	is.NoError(r.Check(buy("BBB", 1, 1), s), "adding to a pending code is allowed")
}

func TestOrderRateLimit(t *testing.T) {
	is := assert.New(t)
	r := risk.OrderRateLimit(2, time.Minute)

	is.NoError(r.Check(buy("AAA", 1, 1), risk.Snapshot{}))
	is.NoError(r.Check(buy("AAA", 1, 1), risk.Snapshot{}))
	is.ErrorIs(r.Check(buy("AAA", 1, 1), risk.Snapshot{}), risk.ErrRateLimited) // 3rd within window
}

func TestManager_PipelineRejectsOnFirstFailure(t *testing.T) {
	is := assert.New(t)
	m := risk.New(risk.MaxOrderValue(dec(500)), risk.MaxPositionPct(0.2))
	s := risk.Snapshot{Balance: dec(100_000)}

	is.NoError(m.Check(buy("AAA", 4, 100), s))
	is.ErrorIs(m.Check(buy("AAA", 10, 100), s), risk.ErrOrderTooBig)
}

func TestManager_CustomFuncRule(t *testing.T) {
	is := assert.New(t)
	boom := errors.New("blocked")
	m := risk.New(risk.Func("custom", func(order.Order, risk.Snapshot) error { return boom }))
	is.ErrorIs(m.Check(buy("AAA", 1, 1), risk.Snapshot{}), boom)
}
