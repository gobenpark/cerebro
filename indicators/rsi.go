package indicators

import (
	"math"

	"github.com/gobenpark/trader/domain"
)

/*
U = 전일 주가가 비교 대상 주가보다 상승했을 때 상승폭
D = 전일 주가가 비교대상 주가보다 하락했을 때 하락폭
AU = 일정기간동안 U의 평균
AD = 일정 기간동안 D의 평균
RS = AU/AD
RSI = AU/(AU+AD) * 100
default
*/

//rsi default period 14
type rsi struct {
	period    int
	indicates []Indicate
}

//NewRsi return new rsi indicator
//default period 14
//parameter period if set 0 then default set 14
func NewRsi(period int) Indicator {
	if period == 0 {
		period = 14
	}
	return &rsi{period: period}
}

func (r *rsi) Calculate(container domain.Container) {
	size := container.Size()
	if size >= r.period {
		slide := size - r.period
		candle := container.Values()

		rscal := func(c []domain.Candle) float64 {
			uv := 0.0
			dv := 0.0
			for _, i := range c {
				v := i.Close - i.Open
				if v > 0 {
					uv += v
				} else {
					dv += math.Abs(v)
				}
			}
			au := uv / float64(len(c))
			du := dv / float64(len(c))
			return au / du
		}

		var indi []Indicate
		for i := 0; i <= slide; i++ {
			rs := rscal(candle[i : r.period+i])
			rsi := 100.0 - 100.0/(1.0+rs)
			indi = append(indi, Indicate{
				Data: rsi,
				Date: candle[i].Date,
			})
		}
		r.indicates = indi
	}
}

func (r *rsi) Get() []Indicate {
	return r.indicates
}

func (r *rsi) PeriodSatisfaction() bool {
	panic("implement me")
}
