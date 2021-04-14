/*
 *                     GNU GENERAL PUBLIC LICENSE
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

type OBV struct {
	obvs []Indicate
}

func NewObv() Indicator {
	return &OBV{obvs: []Indicate{}}
}

func (o *OBV) Calculate(container container.Container) {
	obv := float64(0)
	value := container.Values()
	length := container.Size()

	if len(o.obvs) == 0 {
		for i := length - 2; i >= 0; i-- {
			if value[i].Close > value[i+1].Close {
				obv = obv + value[i].Volume
			} else if value[i].Close < value[i+1].Close {
				obv = obv - value[i].Volume
			}
			o.obvs = append([]Indicate{{
				Data: obv,
				Date: value[i].Date,
			}}, o.obvs...)
		}
	} else {
		if value[0].Close > value[1].Close {
			obv = o.obvs[0].Data + value[0].Volume
		} else if value[0].Close < value[1].Close {
			obv = o.obvs[0].Data - value[0].Volume
		}
		o.obvs = append([]Indicate{{
			Data: obv,
			Date: value[0].Date,
		}})
	}
}

func (o *OBV) Get() []Indicate {
	return o.obvs
}
