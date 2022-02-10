/*
 *  Copyright 2021 The Trader Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package container

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Compress struct {
	StartTime time.Time
	EndTime   time.Time
	*cron.Cron
	mu sync.Mutex
}

func NewCompress(start, end time.Time) *Compress {
	return &Compress{
		StartTime: start,
		EndTime:   end,
		Cron:      cron.New(),
	}
}

func (c *Compress) CompressTick(tickch <-chan Tick, leftEdge bool) (<-chan Candle, <-chan Tick) {
	candlech := make(chan Candle, 1)
	tich := make(chan Tick, 1)

	registCandle := map[CandleType]*Candle{
		Min: {
			Type: Min,
		},
		Min3: {
			Type: Min3,
		},
		Min5: {
			Type: Min5,
		},
		Min15: {
			Type: Min15,
		},
		Min60: {
			Type: Min60,
		},
		Day: {
			Type: Day,
		},
	}

	compressionDate := func(level time.Duration, date time.Time) time.Time {
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

	go func() {
		defer close(candlech)
		defer close(tich)
		for tick := range tickch {
			tich <- tick
			c.mu.Lock()
			for k, candle := range registCandle {
				candle.Code = tick.Code
				if candle.Open == 0 {
					candle.Open = tick.Price
				}
				candle.Volume += tick.Volume
				candle.Close = tick.Price
				if candle.High < tick.Price {
					candle.High = tick.Price
				}
				if candle.Low > tick.Price || candle.Low == 0 {
					candle.Low = tick.Price
				}
				switch k {
				case Min:
					candle.Date = compressionDate(time.Minute, tick.Date)
				case Min3:
					candle.Date = compressionDate(3*time.Minute, tick.Date)
				case Min5:
					candle.Date = compressionDate(5*time.Minute, tick.Date)
				case Min15:
					candle.Date = compressionDate(15*time.Minute, tick.Date)
				case Min60:
					candle.Date = compressionDate(60*time.Minute, tick.Date)
				case Day:
					candle.Date = compressionDate(24*time.Hour, tick.Date)
				}
			}
			c.mu.Unlock()
		}
	}()

	_, err := c.AddFunc("0/3 * * * *", func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		candlech <- *registCandle[Min3]
		registCandle[Min3] = &Candle{
			Type: Min3,
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	_, err = c.AddFunc("* * * * *", func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		candlech <- *registCandle[Min]
		registCandle[Min] = &Candle{
			Type: Min,
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	_, err = c.AddFunc("0/5 * * * *", func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		candlech <- *registCandle[Min5]
		registCandle[Min5] = &Candle{
			Type: Min5,
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	_, err = c.AddFunc("0/15 * * * *", func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		candlech <- *registCandle[Min15]
		registCandle[Min15] = &Candle{
			Type: Min15,
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	_, err = c.AddFunc("0 * * * *", func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		candlech <- *registCandle[Min60]
		registCandle[Min60] = &Candle{
			Type: Min60,
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}
	_, err = c.AddFunc("0 0 * * *", func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		candlech <- *registCandle[Day]
		registCandle[Day] = &Candle{
			Type: Day,
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	c.Start()
	return candlech, tich
}
