/*
 * Copyright 2023 The Trader Authors
 *
 * Licensed under the GNU General Public License v3.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   <https:fsf.org/>
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package container

import (
	"sync"
)

type Container interface {
	Add(tk ...Tick)
	Candles(candleType CandleType) Candles
	Preload()
	Candle(candleType CandleType, idx int) Candle
}

type container struct {
	Code    string
	buffer  []Tick
	candles map[CandleType]Candles
	mu      sync.RWMutex
}

func (c *container) Preload() {
	//TODO implement me
	panic("implement me")
}

func (c *container) Add(tk ...Tick) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.candles == nil {
		c.candles = map[CandleType]Candles{
			Min: {}, Min3: {}, Min5: {}, Min15: {}, Min60: {}, Day: {},
		}
	}
	for k, v := range c.candles {
		c.candles[k] = ResampleCandle(v, k.Duration(), tk...)
	}
}

// 0 index is closed now
func (c *container) Candles(candleType CandleType) Candles {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.candles[candleType]
}

func (c *container) Candle(candleType CandleType, idx int) Candle {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.candles[candleType]) > idx {
		return c.candles[candleType][idx]
	}
	return Candle{}
}
