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

// container package is store of tick data or candle stick data
package container

import (
	"context"

	"github.com/gobenpark/cerebro/log"
)

type ControlPlane struct {
	containers map[string]Container
	log        log.Logger
}

func NewControlPlane(log log.Logger) *ControlPlane {
	return &ControlPlane{containers: map[string]Container{}, log: log}
}

func (c *ControlPlane) Add(tick <-chan Tick) <-chan Container {

	ch := make(chan Container, 1)
	go func() {
		defer close(ch)
		for tk := range tick {
			c.log.Debug("receive tick",
				"code", tk.Code,
				"price", tk.Price,
				"date", tk.Date,
				"askbid", tk.AskBid,
				"volume", tk.Volume,
			)
			if _, ok := c.containers[tk.Code]; !ok {
				c.containers[tk.Code] = &container{}
			}
			c.containers[tk.Code].Add(tk)
			ch <- c.containers[tk.Code]
		}
	}()
	return ch
}

// which one you make candle type of rasample the time duration
func (d *ControlPlane) calculate() {

}

func (d *ControlPlane) merge(ctx context.Context, ch <-chan Tick) {
}
