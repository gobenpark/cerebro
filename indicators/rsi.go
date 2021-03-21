package indicators

import (
	"math"

	"github.com/gobenpark/trader/domain"
)

type rsi struct {
	period    int
	indicates []Indicate
	AD        []Indicate
	AU        []Indicate
}

func NewRsi(period int) Indicator {
	if period == 0 {
		period = 14
	}
	return &rsi{period: period, indicates: []Indicate{}}
}

//self.line[0] = self.line[-1] * self.alpha1 + self.data[0] * self.alpha
func (r *rsi) Calculate(container domain.Container) {
	c := container.Values()
	slide := len(c) - r.period
	if len(c) < r.period {
		return
	}
	alpha := 1.0 / float64(r.period)
	alpha1 := 1.0 - alpha
	aprev := 0.0
	uprev := 0.0

	if v := c[slide].Close - c[slide+1].Close; v > 0 {
		aprev = v
	} else {
		uprev = math.Abs(v)
	}

	var a []float64
	var b []float64

	for i := slide - 1; i >= 0; i-- {
		if v := c[i].Close - c[i+1].Close; v >= 0 {
			aprev = aprev*alpha1 + v*alpha
			a = append([]float64{aprev}, a...)

			uprev = uprev * alpha1
			b = append([]float64{uprev}, b...)
		} else {
			aprev = aprev * alpha1
			a = append([]float64{aprev}, a...)

			uprev = uprev*alpha1 + math.Abs(v)*alpha
			b = append([]float64{uprev}, b...)
		}

		rs := aprev / uprev
		rsi := 100.0 - 100.0/(1.0+rs)
		r.indicates = append([]Indicate{{
			Data: rsi,
			Date: c[i].Date,
		}}, r.indicates...)
	}
}

func (r *rsi) Get() []Indicate {
	return r.indicates
}

func (r *rsi) PeriodSatisfaction() bool {
	panic("implement me")
}
