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
	"sync"
	"time"
)

type InMemoryContainer struct {
	mu      sync.Mutex
	ticks   []Tick
	candles map[time.Duration][]Candle
	code    string
}

func NewInMemoryContainer(code string) *InMemoryContainer {
	return &InMemoryContainer{
		ticks:   nil,
		candles: make(map[time.Duration][]Candle),
		code:    code,
	}
}

func (t *InMemoryContainer) compressDate(date time.Time, level time.Duration, leftEdge bool) time.Time {
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

func (t *InMemoryContainer) AddCandle(candle Candle, tick Tick) Candle {
	candle.Code = tick.Code
	if candle.Open == 0 {
		candle.Open = tick.Price
	}

	if candle.High < tick.Price {
		candle.High = tick.Price
	}

	if candle.Low > tick.Price {
		candle.Low = tick.Price
	}

	candle.Volume += tick.Volume

	candle.Close = tick.Price
	return candle
}

func (t *InMemoryContainer) AppendTick(tick Tick) {

	if len(t.ticks) == 0 {
		t.ticks = append(t.ticks, tick)
		return
	}

	tk := t.ticks[len(t.ticks)-1]
	if tick.Date.Before(tk.Date) {
		return
	}

	t.ticks = append(t.ticks, tick)

	for k, v := range t.candles {
		if len(v) > 0 {
			c := v[len(v)-1]
			tcDate := t.compressDate(tick.Date, k, true)
			switch {
			case c.Date.Equal(tcDate):
				t.candles[k][len(v)-1] = t.AddCandle(c, tick)
			case c.Date.Before(tcDate):
				cd := Candle{
					Code:   tick.Code,
					Open:   tick.Price,
					High:   tick.Price,
					Low:    tick.Price,
					Close:  tick.Price,
					Volume: tick.Volume,
					Date:   tcDate,
				}
				t.candles[k] = append(t.candles[k], cd)
			}
		}

	}
}

func (t *InMemoryContainer) Candles(level time.Duration) []Candle {
	if _, ok := t.candles[level]; ok {
		return t.candles[level]
	}

	candles := ReSample(t.ticks, level, true)
	tCandles := make([]Candle, len(candles))
	copy(tCandles, candles)
	t.candles[level] = candles
	return tCandles
}

func (t *InMemoryContainer) Code() string {
	return t.code
}
