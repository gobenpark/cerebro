/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
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
