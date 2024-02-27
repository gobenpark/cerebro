/*
 *  Copyright 2021 The Cerebro Authors
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
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

type Position struct {
	Item  *item.Item
	Size  int64 `json:"size"`
	Price int64 `json:"price"`
}

func NewPosition(o order.Order) Position {
	return Position{
		Item:  o.Item(),
		Size:  o.Size(),
		Price: o.Price(),
	}
}
