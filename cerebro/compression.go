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
package cerebro

import (
	"time"

	"github.com/gobenpark/trader/container"
)

type CompressInfo struct {
	level    time.Duration
	LeftEdge bool
}

func Compression(tick <-chan container.Tick, level time.Duration, leftEdge bool) <-chan container.Candle {
	compressionDate := func(date time.Time) time.Time {
		rd := date.Round(level)
		if leftEdge {
			if date.Sub(rd) < 0 {
				rd = rd.Add(-level)
			}
		} else {
			if date.Sub(rd) > 0 {
				rd = rd.Add(level)
			}
		}
		return rd
	}
	ch := make(chan container.Candle, 1)
	go func() {
		defer close(ch)
		c := container.Candle{}
		for t := range tick {
			if c.Date.Equal(time.Time{}) {
				c.Date = compressionDate(t.Date)
			}

			if c.Date.Equal(compressionDate(t.Date)) {
				c.Volume += t.Volume
				c.Code = t.Code
				c.Close = t.Price
				if c.Open == 0 {
					c.Open = t.Price
				}

				if c.High < t.Price {
					c.High = t.Price
				}

				if c.Low == 0 || c.Low > t.Price {
					c.Low = t.Price
				}
			} else {
				ch <- c
				c = container.Candle{}
			}
		}
	}()
	return ch
}
