/*
 *  Copyright 2021 The Cerebro Authors
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

package indicator

import (
	"sort"
	"sync"
	"time"

	e "github.com/gobenpark/cerebro/error"
)

var (
	ErrOverDate = e.Error{Code: 1, Message: "raise unexpected error"}
)

// Rasampler is resample for realtime tick data
func Resampler(last *Candle, tk Tick, compress time.Duration) error {
	d := last.Date.Add(compress).Truncate(compress)

	if tk.Date.After(d) {
		return ErrOverDate
	}
	last.Close = tk.Price
	last.Volume += tk.Volume
	if last.Low > tk.Price {
		last.Low = tk.Price
	}
	if last.High < tk.Price {
		last.High = tk.Price
	}
	return nil
}

func ResampleCandle(compress time.Duration, tick ...Tick) Candle {
	one := sync.Once{}
	candle := Candle{}

	for i := range tick {
		one.Do(func() {
			candle = Candle{
				Type:   0,
				Open:   tick[i].Price,
				High:   tick[i].Price,
				Low:    tick[i].Price,
				Close:  tick[i].Price,
				Volume: tick[i].Volume,
				Date:   tick[i].Date.Truncate(compress),
			}
		})

		if candle.Open == 0 {
			candle.Open = tick[i].Price
		}
		candle.Date = tick[i].Date.Truncate(compress)
		if candle.Low > tick[i].Price {
			candle.Low = tick[i].Price
		}

		if candle.High < tick[i].Price {
			candle.High = tick[i].Price
		}

		candle.Close = tick[i].Price
		candle.Volume += tick[i].Volume
	}
	return candle
}

func Resample(tk []Tick, compress time.Duration) Candles {
	sort.Slice(tk, func(i, j int) bool {
		return tk[i].Date.Before(tk[j].Date)
	})
	cds := Candles{}
	for i := range tk {
		if len(cds) == 0 {
			cds = append(cds, Candle{
				Open:   tk[i].Price,
				High:   tk[i].Price,
				Low:    tk[i].Price,
				Close:  tk[i].Price,
				Date:   tk[i].Date.Truncate(compress),
				Volume: tk[i].Volume,
			})
			continue
		}
		last := cds[len(cds)-1]
		edge := last.Date.Add(compress).Truncate(compress)
		if tk[i].Date.Before(edge) {
			last.Close = tk[i].Price
			last.Volume += tk[i].Volume

			if last.Low > tk[i].Price {
				last.Low = tk[i].Price
			}

			if last.High < tk[i].Price {
				last.High = tk[i].Price
			}
			cds[len(cds)-1] = last
		} else {
			cds = append(cds, Candle{
				Open:   tk[i].Price,
				High:   tk[i].Price,
				Low:    tk[i].Price,
				Close:  tk[i].Price,
				Date:   tk[i].Date.Truncate(compress),
				Volume: tk[i].Volume,
			})
		}
	}
	return cds
}
