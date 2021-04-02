/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
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
