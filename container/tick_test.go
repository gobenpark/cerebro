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

	"github.com/stretchr/testify/require"
)

func TestTick(t *testing.T) {
	tk := Tick{
		Code:   "1",
		AskBid: "ask",
		Date:   time.Now(),
		Price:  20,
		Volume: 1000,
	}
	time.Sleep(1 * time.Second)

	tk2 := Tick{
		Code:   "2",
		AskBid: "bid",
		Date:   time.Now(),
		Price:  10,
		Volume: 2000,
	}
	require.Len(t, ReSample([]Tick{tk, tk2}, time.Second, true), 2)
	require.Len(t, ReSample([]Tick{tk, tk2}, time.Minute, true), 1)
}
