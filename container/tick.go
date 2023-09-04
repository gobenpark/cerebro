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
package container

import (
	"encoding/json"
	"time"
)

type Spread string

const (
	Bid Spread = "bid"
	Ask Spread = "ask"
)

type Tick struct {
	Code      string    `json:"code"`
	AskBid    Spread    `json:"askBid"`
	Date      time.Time `json:"date"`
	Price     int64     `json:"price"`
	AccVolume int64     `json:"acc_volume"`
	Volume    int64     `json:"volume"`
}

func (t *Tick) UnmarshalJSON(bytes []byte) error {
	var data map[string]interface{}
	err := json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}
	t.Code = data["code"].(string)
	ti, err := time.Parse("2006-01-02T15:04:05", data["dt"].(string))
	if err != nil {
		return err
	}

	t.Date = ti
	t.AskBid = data["askBid"].(Spread)
	t.Price = data["price"].(int64)
	t.Volume = data["volume"].(int64)
	return nil
}
