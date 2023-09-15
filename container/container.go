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
	"fmt"

	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Container interface {
	Calculate(tick Tick)
}

type container struct {
	cache       *badger.DB
	mu          sync.Mutex
	Code        string
	buf         []Tick
	off         int
	bufferCount int
	candles     Candles
}

// NewContainer creates a new container length is the buffer length
func NewContainer(cache *badger.DB, code string, length int) Container {
	return &container{
		cache:   cache,
		Code:    code,
		buf:     make([]Tick, length),
		candles: Candles{},
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
	c.candles = CalculateCandle(c.candles, time.Minute, cd)
	fmt.Println(c.candles)
}
