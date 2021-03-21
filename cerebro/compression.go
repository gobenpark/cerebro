package cerebro

import (
	"time"

	"github.com/gobenpark/trader/domain"
)

type CompressInfo struct {
	level    time.Duration
	LeftEdge bool
}

func Compression(tick <-chan domain.Tick, level time.Duration, leftEdge bool) <-chan domain.Candle {
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
	ch := make(chan domain.Candle, 1)
	go func() {
		defer close(ch)
		c := domain.Candle{}
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
				c = domain.Candle{}
			}
		}
	}()
	return ch
}
