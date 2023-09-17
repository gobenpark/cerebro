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
	"errors"
	"fmt"

	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var (
	ErrNotExist = errors.New("candle does not exist")
)

type Container interface {
	Calculate(tick Tick)
}

type container struct {
	cache        *badger.DB
	mu           sync.Mutex
	Code         string
	buf          []Tick
	off          int
	bufferCount  int
	baseCandles  Candles
	candles      map[time.Duration]Candles
	candleOffset map[time.Duration]int
}

// NewContainer creates a new container length is the buffer length
func NewContainer(cache *badger.DB, code string, length int) Container {
	return &container{
		cache:       cache,
		Code:        code,
		buf:         make([]Tick, length),
		baseCandles: Candles{},
	}
}

func currentTick(code string) []byte {
	return []byte(fmt.Sprintf("%s:tick", code))
}

func candle(code string) []byte {
	return []byte(fmt.Sprintf("%s:candle", code))
}

func (c *container) Calculate(tick Tick) {
	if c.off != len(c.buf) {
		c.buf[c.off] = tick
		c.off += 1
		return
	}

	c.off = 0
	cd := Resample(c.buf, time.Minute)

	c.baseCandles = CalculateCandle(c.baseCandles, time.Minute, cd)
	c.postHook()
}

func (c *container) Preload(candles Candles) {
	c.baseCandles = candles
}

func (c *container) Candle(duration time.Duration, index int) (Candle, error) {
	if v, ok := c.candles[duration]; ok {
		idx := (v.Len() - index) - 1
		return v[idx], nil
	}
	return Candle{}, ErrNotExist
}

// generate other candle
func (c *container) postHook() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.candles {
		c.candles[k] = CalculateCandle(v, k, c.baseCandles[c.candleOffset[k]:])
		c.candleOffset[k] = c.baseCandles.Len() - 1
	}
}
