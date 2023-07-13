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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestContainer_Add(t *testing.T) {
	c := container{
		Code: "005930",
		mu:   sync.RWMutex{},
	}

	c.add(
		Tick{
			Code:   "005930",
			AskBid: Bid,
			Date:   time.Now().Add(time.Second),
			Price:  10000,
			Volume: 10,
		},
		Tick{
			Code:   "005930",
			AskBid: Ask,
			Date:   time.Now().Add(2 * time.Second),
			Price:  10002,
			Volume: 2,
		},
		Tick{
			Code:   "005930",
			AskBid: Ask,
			Date:   time.Now().Add(3 * time.Second),
			Price:  10003,
			Volume: 2,
		},
		Tick{
			Code:   "005930",
			AskBid: Ask,
			Date:   time.Now().Add(4 * time.Second),
			Price:  10043,
			Volume: 2,
		},
		Tick{
			Code:   "005930",
			AskBid: Ask,
			Date:   time.Now().Add(4 * time.Second),
			Price:  10023,
			Volume: 2,
		},
	)
	result := c.Candle(Day)
	require.Len(t, result, 1)

	require.Equal(t, result[0].Open, int64(10000))
	require.Equal(t, result[0].High, int64(10043))
	require.Equal(t, result[0].Open, int64(10000))
	require.Equal(t, result[0].Close, int64(10023))
}
