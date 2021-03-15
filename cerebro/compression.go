package cerebro

import (
	"reflect"
	"time"

	"github.com/gobenpark/trader/domain"
)

type CompressInfo struct {
	level time.Duration
}

//TODO: 거래량이 없을경우 정해진 기간마다 빈 candle 전송
func Compression(tick <-chan domain.Tick, level time.Duration) <-chan domain.Candle {
	ch := make(chan domain.Candle, 1)
	go func() {
		defer close(ch)
		c := domain.Candle{}
		for t := range tick {
			if c.Date.Equal(time.Time{}) || t.Date.After(c.Date) {
				if !reflect.DeepEqual(c, domain.Candle{}) {
					ch <- c
				}
				rounded := t.Date.Round(level)
				if t.Date.Sub(rounded) > 0 {
					rounded = rounded.Add(level)
				}

				c.Date = rounded
			}
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
		}
	}()
	return ch
}
