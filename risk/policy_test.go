package risk

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func d(v int64) decimal.Decimal { return decimal.NewFromInt(v) }

func TestPolicy_Enabled(t *testing.T) {
	is := assert.New(t)
	is.False(Policy{}.Enabled(), "no trigger set")
	is.True(Policy{StopLoss: 0.01}.Enabled())
	is.True(Policy{TrailingStop: 0.01}.Enabled())
	is.True(Policy{TakeProfit: 0.01}.Enabled())
}

func TestPolicy_Triggered(t *testing.T) {
	is := assert.New(t)
	entry := d(100)

	// Stop-loss at 5% exits at or below 95.
	sl := Policy{StopLoss: 0.05}
	_, ok := sl.triggered(entry, entry, d(96))
	is.False(ok, "above the stop")
	reason, ok := sl.triggered(entry, entry, d(95))
	is.True(ok)
	is.Equal("stop-loss", reason)

	// Take-profit at 10% exits at or above 110.
	tp := Policy{TakeProfit: 0.10}
	_, ok = tp.triggered(entry, entry, d(109))
	is.False(ok, "below the target")
	reason, ok = tp.triggered(entry, entry, d(110))
	is.True(ok)
	is.Equal("take-profit", reason)

	// Trailing-stop at 5% off a peak of 120 exits at or below 114.
	tr := Policy{TrailingStop: 0.05}
	_, ok = tr.triggered(entry, d(120), d(115))
	is.False(ok, "above the trailing stop")
	reason, ok = tr.triggered(entry, d(120), d(114))
	is.True(ok)
	is.Equal("trailing-stop", reason)

	// Stop-loss is checked before take-profit.
	both := Policy{StopLoss: 0.05, TakeProfit: 0.10}
	reason, ok = both.triggered(entry, entry, d(95))
	is.True(ok)
	is.Equal("stop-loss", reason)

	// An unknown (zero) entry price cannot be evaluated.
	_, ok = sl.triggered(decimal.Zero, decimal.Zero, d(1))
	is.False(ok)

	// A policy with no trigger never fires.
	_, ok = Policy{}.triggered(entry, entry, d(1))
	is.False(ok)
}
