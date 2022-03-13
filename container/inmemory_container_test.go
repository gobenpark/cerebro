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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTempContainer_AddTicks(t *testing.T) {
	c := NewInMemoryContainer("KRW-BTC")

	ti := time.Now()
	t1 := Tick{
		Code:   "KRW-BTC",
		AskBid: "bid",
		Date:   ti.Add(time.Minute),
		Price:  1,
		Volume: 20,
	}
	c.AppendTick(t1)

	fmt.Println(c.Candles(time.Minute))

	t2 := Tick{
		Code:   "KRW-BTC",
		AskBid: "bid",
		Date:   ti.Add(2 * time.Minute),
		Price:  2,
		Volume: 20,
	}

	c.AppendTick(t2)
	t3 := Tick{
		Code:   "KRW-BTC",
		AskBid: "bid",
		Date:   ti.Add(3 * time.Minute),
		Price:  3,
		Volume: 20,
	}
	c.AppendTick(t3)

	t4 := Tick{
		Code:   "KRW-BTC",
		AskBid: "bid",
		Date:   ti.Add(3 * time.Minute).Add(time.Second),
		Price:  2,
		Volume: 20,
	}
	c.AppendTick(t4)

	t5 := Tick{
		Code:   "KRW-BTC",
		AskBid: "bid",
		Date:   ti.Add(3 * time.Minute).Add(2 * time.Second),
		Price:  5,
		Volume: 30,
	}
	c.AppendTick(t5)

	t3t4t5 := c.Candles(time.Minute)

	require.Len(t, t3t4t5, 3)
	require.Equal(t, t3t4t5[len(t3t4t5)-1].Open, float64(3))
	require.Equal(t, t3t4t5[len(t3t4t5)-1].High, float64(5))
	require.Equal(t, t3t4t5[len(t3t4t5)-1].Low, float64(2))
	require.Equal(t, t3t4t5[len(t3t4t5)-1].Close, float64(5))
	require.Equal(t, t3t4t5[len(t3t4t5)-1].Volume, float64(70))
}
