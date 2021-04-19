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
package strategy

import (
	"context"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	*broker.Broker
	Sts []Strategy
}

func (s *Engine) Start(ctx context.Context, data chan container.Container) {
	go func() {
	Done:
		for {
			select {
			case i := <-data:
				for _, st := range s.Sts {
					st.Next(s.Broker, i)
				}
			case <-ctx.Done():
				break Done
			}
		}
	}()
}

func (s *Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case *order.Order:
		for _, strategy := range s.Sts {
			strategy.NotifyOrder(et)
		}
	}
}
