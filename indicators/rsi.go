package indicators

import (
	"fmt"

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
	U         []Indicate
	D         []Indicate
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
		var uvalue []Indicate
		var dvalue []Indicate

		calcu := func(i int) {
			v := candle[i].Close - candle[i+1].Close
			if v > 0 {
				uvalue = append(uvalue, Indicate{
					Data: v,
					Date: candle[i].Date,
				})
				dvalue = append(dvalue, Indicate{
					Data: 0,
					Date: candle[i].Date,
				})
			} else {
				uvalue = append(uvalue, Indicate{
					Data: 0,
					Date: candle[i].Date,
				})
				dvalue = append(dvalue, Indicate{
					Data: -v,
					Date: candle[i].Date,
				})
			}
		}
		fmt.Println(size)
		fmt.Println(len(r.U))
		fmt.Println(len(candle))
		for i := 0; i < size-1; i++ {
			if len(r.U) != 0 {
				if r.U[0].Date.Before(candle[i].Date) {
					calcu(i)
				} else {
					break
				}
			} else {
				calcu(i)
			}
		}
		r.U = append(uvalue, r.U...)
		r.D = append(dvalue, r.D...)
		var indi []Indicate
		for i := 0; i <= slide-1; i++ {
			avg := func(s []Indicate) float64 {
				value := float64(0)
				for _, v := range s {
					value += v.Data
				}
				return value / float64(len(s))
			}

			AU := avg(r.U[i : r.period+i])
			AD := avg(r.D[i : r.period+i])
			rs := AU / AD
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
