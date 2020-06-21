package util

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSMA(t *testing.T) {
	data := []float64{50600, 50400, 52800, 53500, 53900, 53300, 53000, 55100, 55000, 54200}

	require.Equal(t, float64(52240), SMA(data, 5)[0])
}
