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
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Sample struct {
	data string
}

func TestLeftInsert(t *testing.T) {
	d := Sample{data: "dsad"}

	bt, err := json.Marshal(d)
	require.NoError(t, err)

	b := bytes.Buffer{}

	n, err := b.Write(bt)
	require.NoError(t, err)
	fmt.Println(n)
}

func BenchmarkContainer(b *testing.B) {
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
	defer db.Close()
	assert.NoError(b, err)
	benchmarks := []struct {
		name      string
		container Container
	}{
		{
			"default",
			NewDataContainer(Info{
				Code: "default",
			}),
		},
		{
			"badger",
			NewBadgerContainer(db, Info{
				Code: "badger",
			}),
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.container.Add(Candle{
					Code:   "default",
					Open:   1,
					High:   2,
					Low:    1000,
					Close:  3000,
					Volume: 1000000,
					Date:   time.Now(),
				})

				bm.container.Values()
			}
		})
	}
}
