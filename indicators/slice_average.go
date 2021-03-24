package indicators

import "github.com/gobenpark/trader/container"

func average(candle []container.Candle) float64 {
	total := 0.0
	for _, v := range candle {
		total += v.Close
	}
	return total / float64(len(candle))
}
