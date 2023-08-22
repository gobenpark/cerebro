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
package container

import (
	"errors"
	"io"
	"time"
)

type readOp int8
type CandleType int

const (
	opRead      readOp = -1 // Any other read operation.
	opInvalid   readOp = 0  // Non-read operation.
	opReadRune1 readOp = 1  // Read rune of size 1.
	opReadRune2 readOp = 2  // Read rune of size 2.
	opReadRune3 readOp = 3  // Read rune of size 3.
	opReadRune4 readOp = 4  // Read rune of size 4.
)

const (
	Min CandleType = iota + 1
	Min3
	Min5
	Min15
	Min60
	Day
)

func (c CandleType) Duration() time.Duration {
	switch c {
	case Min:
		return time.Minute
	case Min3:
		return 3 * time.Minute
	case Min5:
		return 5 * time.Minute
	case Min15:
		return 15 * time.Minute
	case Min60:
		return time.Hour
	case Day:
		return 24 * time.Hour
	default:
		return 0
	}
}

const maxInt = int(^uint(0) >> 1)

type Candle struct {
	Type   CandleType
	Code   string    `json:"code"`
	Open   int64     `json:"open"`
	High   int64     `json:"high"`
	Low    int64     `json:"low"`
	Close  int64     `json:"close"`
	Volume int64     `json:"volume"`
	Date   time.Time `json:"date"`
}

type TradeHistory struct {
	Code        string    `json:"code"`
	Price       float64   `json:"price"`
	Volume      float64   `json:"volume"`
	PrevPrice   float64   `json:"prevPrice"`
	ChangePrice float64   `json:"changePrice"`
	ASKBID      string    `json:"askbid"`
	Date        time.Time `json:"date"`
	ID          int64     `json:"id"`
}

type CandleBuffer struct {
	buf      []Candle
	off      int
	lastRead readOp
}

func NewCandleBuffer(c []Candle) *CandleBuffer {
	return &CandleBuffer{buf: c}
}

func (c *CandleBuffer) Less(i, j int) bool {
	return c.buf[i].Date.Before(c.buf[j].Date)
}

func (c *CandleBuffer) Swap(i, j int) {
	c.buf[i], c.buf[j] = c.buf[j], c.buf[i]
}

func (c *CandleBuffer) empty() bool {
	return len(c.buf) <= c.off
}

func (c *CandleBuffer) Len() int {
	return len(c.buf) - c.off
}

func (c *CandleBuffer) Cap() int {
	return cap(c.buf)
}

func (c *CandleBuffer) Truncate(n int) {
	if n == 0 {
		c.Reset()
		return
	}
	c.lastRead = opInvalid
	if n < 0 || n > c.Len() {
		panic("Candle Buffer: truncation out of range")
	}
	c.buf = c.buf[:c.off+n]
}

func (c *CandleBuffer) Reset() {
	c.buf = c.buf[:0]
	c.off = 0
	c.lastRead = opInvalid
}

func (c *CandleBuffer) Write(p []Candle) (n int, err error) {
	c.lastRead = opInvalid
	m, ok := c.tryGrowByReslice(len(p))
	if !ok {
		m = c.grow(len(p))
	}
	return copy(c.buf[m:], p), nil
}

func makeSlice(n int) []Candle {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(errors.New("candle Buffer: too large"))
		}
	}()
	return make([]Candle, n)
}

func (c *CandleBuffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(c.buf); n <= cap(c.buf)-l {
		c.buf = c.buf[:l+n]
		return l, true
	}
	return 0, false
}

func (c *CandleBuffer) grow(n int) int {
	m := c.Len()
	// If buffer is empty, reset to recover space.
	if m == 0 && c.off != 0 {
		c.Reset()
	}
	// Try to grow by means of a reslice.
	if i, ok := c.tryGrowByReslice(n); ok {
		return i
	}
	if c.buf == nil && n <= 64 {
		c.buf = make([]Candle, n, 64)
		return 0
	}
	b := cap(c.buf)
	if n <= b/2-m {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= c to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		copy(c.buf, c.buf[c.off:])
	} else if b > maxInt-b-n {
		panic(errors.New("candle Buffer: too large"))
	} else {
		// Not enough space anywhere, we need to allocate.
		buf := makeSlice(2*b + n)
		copy(buf, c.buf[c.off:])
		c.buf = buf
	}
	// Restore b.off and len(b.buf).
	c.off = 0
	c.buf = c.buf[:m+n]
	return m
}

func (c *CandleBuffer) Next(n int) []Candle {
	c.lastRead = opInvalid
	m := c.Len()
	if n > m {
		n = m
	}
	data := c.buf[c.off : c.off+n]
	c.off += n
	if n > 0 {
		c.lastRead = opRead
	}
	return data
}

func (c *CandleBuffer) Read(p []Candle) (n int, err error) {
	c.lastRead = opInvalid
	if c.empty() {
		c.Reset()
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n = copy(p, c.buf[c.off:])
	c.off += n
	if n > 0 {
		c.lastRead = opRead
	}
	return n, nil
}
