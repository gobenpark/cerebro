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

type Container interface {
	Calculate(tick Tick)
}

type container struct {
	cache *badger.DB
	mu    sync.Mutex
	Code  string
	buf   []Tick
	off   int
}

func NewContainer(cache *badger.DB, code string) Container {
	return &container{
		cache: cache,
		Code:  code,
		buf:   make([]Tick, 100),
	}
}

func currentTick(code string) []byte {
	return []byte(fmt.Sprintf("%s:tick", code))
}

func (c *container) Preload() {

}

func (c *container) Calculate(tick Tick) {
	if c.off != 100 {
		c.buf[c.off] = tick
		c.off += 1
		return
	}

	c.off = 0
	cd := Resample(c.buf, time.Minute)
	txn := c.cache.NewTransaction(true)
	defer txn.Discard()

	var bt []byte

	it, err := txn.Get(currentTick(c.Code))
	if err != nil && errors.Is(err, badger.ErrKeyNotFound) {
		bt, err = json.Marshal(cd[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := txn.Set(currentTick(c.Code), bt); err != nil {
			fmt.Println(err)
			return
		}
	} else if err != nil {
		fmt.Println(err)
	} else {
		it.Value(func(val []byte) error {
			bt = val
			return nil
		})
	}

	var tk Tick
	if err := json.Unmarshal(bt, &tk); err != nil {
		fmt.Println(err)
	}

	switch len(cd) {
	case 1:

	case 2:

	}
}
