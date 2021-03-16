package indicators

import (
	"github.com/gobenpark/trader/domain"
)

type sma struct {
	period    int
	indicates []Indicate
}

func NewSma(period int) Indicator {
	return &sma{period: period}
}

func (s *sma) Set(container domain.Container) {
	size := container.Size()
	if size >= s.period {
		slide := (size - s.period) + 1
		candle := container.Values()

		for i := 0; i < slide; i++ {
			id := Indicate{
				Data: average(candle[i : s.period+i]),
				Date: candle[i].Date,
			}
			s.indicates = append(s.indicates, id)
		}
	}
}

func (s *sma) Get() []Indicate {
	return s.indicates
}

func (s *sma) PeriodSatisfaction() bool {
	return len(s.indicates) > s.period
}

func average(candle []domain.Candle) float64 {
	total := 0.0
	for _, v := range candle {
		total += v.Close
	}
	return total / float64(len(candle))
}
