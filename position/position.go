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
package position

import (
	"time"

	"github.com/gobenpark/cerebro/order"
)

type Position struct {
	Code      string    `json:"code"`
	Size      int64     `json:"size"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewPosition(o order.Order) Position {
	return Position{
		Code:      o.Code(),
		Size:      o.Size(),
		Price:     o.Price(),
		CreatedAt: time.Now(),
	}
}
