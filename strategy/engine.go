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
	"bytes"
	"context"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	broker.Broker
	st  Strategy
	buf <-chan container.Candle

	candles    []container.Candle
	indicators map[indicators.IndicatorType][]indicators.Indicate
}

func NewEngine(bk broker.Broker, st Strategy, buf <-chan container.Candle) Engine {

	return Engine{
		Broker: bk,
		st:     st,
		buf:    buf,
	}
}

func (s Engine) Start(ctx context.Context) {
	go func() {
	Done:
		for {
			select {

			case <-ctx.Done():
				break Done
			}
		}
	}()

	bytes.Buffer{}
}

func (s Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case *order.Order:
		s.st.NotifyOrder(et)
	}
}
