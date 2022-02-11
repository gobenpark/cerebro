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
	"time"
)

type CompressInfo struct {
	Level    time.Duration
	LeftEdge bool
}

func Compression(tick <-chan Tick, level time.Duration, leftEdge bool) <-chan Candle {
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
	ch := make(chan Candle, 1)
	go func() {
		defer close(ch)
		c := Candle{}
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
				c = Candle{}
			}
		}
	}()
	return ch
}
