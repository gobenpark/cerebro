package util

func average(xs []float64) float64 {
	total := 0.0
	for _, v := range xs {
		total += v
	}
	return total / float64(len(xs))
}

func SMA(data []float64, period int) []float64 {
	if len(data) < period {
		return []float64{}
	}

	slide := (len(data) - period) + 1
	result := []float64{}
	for i := 0; i < slide; i++ {
		result = append(result, average(data[i:period+i]))
	}

	return result
}
