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
	"sort"
	"sync"
	"time"
)

//TODO: 기존 히스토리 데이터 추가
type TempContainer struct {
	mu    sync.Mutex
	ticks []Tick
	code  string
}

func NewTempContainer(code string) *TempContainer {
	return &TempContainer{ticks: []Tick{}, code: code}
}

func (t *TempContainer) AddTicks(ticks ...Tick) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ticks = append(t.ticks, ticks...)
	sort.Slice(t.ticks, func(i, j int) bool {
		return t.ticks[i].Date.Before(t.ticks[j].Date)
	})
}

func (t *TempContainer) Candles(level time.Duration) []Candle {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ReSample(t.ticks, level, true)
}
