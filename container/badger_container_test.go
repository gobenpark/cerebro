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
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
)

func TestBgContainer_Value(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
	defer db.Close()
	assert.NoError(t, err)
	c := NewBadgerContainer(db, Info{
		Code:             "KRW-BTC",
		CompressionLevel: 0,
	})

	c.Add(Candle{
		Code:   "KRW-BTC",
		Open:   1,
		High:   2,
		Low:    3,
		Close:  4,
		Volume: 10,
		Date:   time.Now(),
	})
	c.Add(Candle{
		Code:   "KRW-BTC",
		Open:   1,
		High:   2,
		Low:    3,
		Close:  4,
		Volume: 11,
		Date:   time.Now(),
	})

	v := c.Values()
	assert.NotNil(t, v)
	assert.Len(t, v, 2)
	assert.Equal(t, float64(11), v[0].Volume)
}
