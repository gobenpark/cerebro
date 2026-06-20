package indicator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gobenpark/cerebro/indicator"
)

// TestRma checks the running moving average used by the RSI calculation:
// it is a simple average until the window fills, then a Wilder-style smoothing.
func TestRma(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	out := indicator.Rma(2, []float64{2, 4, 6})
	must.Len(out, 3)

	is.InDelta(2.0, out[0], 1e-9) // i<period: sum 2 / 1
	is.InDelta(3.0, out[1], 1e-9) // i<period: (2+4) / 2
	is.InDelta(4.5, out[2], 1e-9) // smoothing: (3*1 + 6) / 2
}

func TestRma_DefaultsToInputLength(t *testing.T) {
	is := assert.New(t)
	is.Empty(indicator.Rma(3, nil))
}
