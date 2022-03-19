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

package analysis

import (
	"fmt"
	"sync"

	"github.com/gobenpark/trader/order"
)

type InmemoryAnalyzer struct {
	InitValue float64
	mu        sync.Mutex
}

func NewInmemoryAnalyzer() *InmemoryAnalyzer {

	return &InmemoryAnalyzer{InitValue: 400000}
}

func (a *InmemoryAnalyzer) Listen(e interface{}) {

	switch o := e.(type) {
	case order.Order:
		if o.Action() == order.Sell {
			a.mu.Lock()
			if o.Status() == order.Completed {

				a.InitValue += (o.OrderPrice() - (o.OrderPrice()*o.Commission())/100)

				fmt.Printf("%s팔았다,%f원에 %d만큼 상태는? %d 남은돈: %f\n", o.Code(), o.Price(), o.Size(), o.Status(), a.InitValue)
			}
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			if o.Status() == order.Completed {
				a.InitValue -= (o.OrderPrice() + (o.OrderPrice()*o.Commission())/100)
				fmt.Printf("%s샀다,%f원에 %d만큼 상태는? %d 남은돈: %f\n", o.Code(), o.Price(), o.Size(), o.Status(), a.InitValue)
			}
			a.mu.Unlock()
		}
	}
}
