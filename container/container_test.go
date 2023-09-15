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
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestContainer_Add(t *testing.T) {
	cache, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
	require.NoError(t, err)
	c := NewContainer(cache, "005930", 5)

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(time.Second),
		Price:  100,
		Volume: 100,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(3 * time.Second),
		Price:  200,
		Volume: 200,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(2 * time.Second),
		Price:  300,
		Volume: 300,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(5 * time.Second),
		Price:  400,
		Volume: 400,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(time.Minute),
		Price:  500,
		Volume: 500,
	})
	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(2 * time.Minute),
		Price:  600,
		Volume: 600,
	})

	time.Sleep(10 * time.Second)
	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(time.Second),
		Price:  100,
		Volume: 100,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(3 * time.Second),
		Price:  200,
		Volume: 200,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(2 * time.Second),
		Price:  300,
		Volume: 300,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(5 * time.Second),
		Price:  400,
		Volume: 400,
	})

	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(time.Minute),
		Price:  500,
		Volume: 500,
	})
	c.Calculate(Tick{
		Code:   "005930",
		AskBid: "ask",
		Date:   time.Now().Add(2 * time.Minute),
		Price:  600,
		Volume: 600,
	})
}