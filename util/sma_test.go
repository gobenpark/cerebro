package util

import (
	"fmt"
	"testing"
)

func TestSMA(t *testing.T) {
	data := []float64{50600, 50400, 52800, 53500, 53900, 53300, 53000, 55100, 55000, 54200}

	fmt.Println(SMA(data, 5))
}
