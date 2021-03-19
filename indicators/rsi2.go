package indicators

import (
	"math"

	"github.com/gobenpark/trader/domain"
)

//첫번째 AU/AD 계산
//- AU : 지난 14일 동안의 이득의 합 / 14
//- AD : 지난 14일 동안의 하락의 합 / 14
//
//두번째 및 이후의 AU/AD 계산
//- AU : [(이전 AU) * 13 + 현재 이득] / 14
//- AD : [(이전 AD) * 13 + 현재 하락] / 14
//
//RS = AU / AD
//RSI = RS / (1 + RS) 또는 RSI = AU / (AU + AD)
type rsi2 struct {
	period    int
	indicates []Indicate
	AD        []Indicate
	AU        []Indicate
}

func NewRsi2(period int) Indicator {
	if period == 0 {
		period = 14
	}
	return &rsi2{period: period}
}

func upday(c []domain.Candle) float64 {
	value := 0.0
	for _, i := range c {
		if v := i.Close - i.Open; v > 0 {
			value += v
		}
	}
	return value
}

func downday(c []domain.Candle) float64 {
	value := 0.0
	for _, i := range c {
		if v := i.Close - i.Open; v < 0 {
			value += math.Abs(v)
		}
	}
	return value
}

func (r *rsi2) Calculate(container domain.Container) {
	c := container.Values()
	slide := len(c) - r.period
	if len(c) < r.period {
		return
	}

	AU := math.NaN()
	AD := math.NaN()

	for i := slide; i >= 0; i-- {
		if math.IsNaN(AU) && math.IsNaN(AD) {
			su := upday(c[i : i+r.period])
			sd := downday(c[i : i+r.period])
			AU = su / float64(r.period)
			AD = sd / float64(r.period)
		} else {
			if v := c[i].Close - c[i].Open; v > 0 {
				AU = ((AU * float64(r.period-1)) + v) / float64(r.period)
				AD = AD * float64(r.period-1)
			} else {
				AD = ((AD * float64(r.period-1)) + math.Abs(v)) / float64(r.period)
				AU = AU * float64(r.period-1)
			}
		}
		rs := AU / AD
		r.indicates = append([]Indicate{{
			Data: 100.0 - 100.0/(1.0+rs),
			Date: c[i].Date,
		}})
	}
}

func (r *rsi2) Get() []Indicate {
	return r.indicates
}

func (r *rsi2) PeriodSatisfaction() bool {
	panic("implement me")
}
