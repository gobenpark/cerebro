/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */
package indicators

import (
	"github.com/gobenpark/trader/container"
)

type sma struct {
	period    int
	indicates []Indicate
}

func NewSma(period int) Indicator {
	return &sma{period: period}
}

func (s *sma) Calculate(container container.Container) {
	size := container.Size()
	var indicates []Indicate
	if size >= s.period {
		slide := (size - s.period)
		candle := container.Values()

		for i := 0; i <= slide; i++ {
			id := Indicate{
				Data: average(candle[i : s.period+i]),
				Date: candle[i].Date,
			}

			if len(s.indicates) != 0 {
				if id.Date.After(s.indicates[0].Date) {
					indicates = append(indicates, id)
					continue
				}
				break
			} else {
				indicates = append(indicates, id)
			}
		}
		s.indicates = append(indicates, s.indicates...)
	}
}

func (s *sma) Get() []Indicate {
	return s.indicates
}

func (s *sma) PeriodSatisfaction() bool {
	return len(s.indicates) > s.period
}
