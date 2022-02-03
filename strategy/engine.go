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
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/log"
	"github.com/gobenpark/trader/order"
)

type Engine struct {
	broker.Broker
	st  Strategy
	log log.Logger

	indicators map[indicators.IndicatorType][]indicators.Indicate
	containers map[container.CandleType]container.Container
}

func NewEngine(log log.Logger, bk broker.Broker, st Strategy) *Engine {

	return &Engine{
		Broker:     bk,
		st:         st,
		containers: map[container.CandleType]container.Container{},
		log:        log,
	}
}

func (s *Engine) Spawn(ctx context.Context, candle <-chan container.Candle, tick <-chan container.Tick) {

Done:
	for {
		select {
		case <-ctx.Done():
			break Done
		case cd := <-candle:
			if _, ok := s.containers[cd.Type]; !ok {
				s.containers[cd.Type] = container.NewDataContainer(container.Info{
					Code: cd.Code,
				})
			}

			s.containers[cd.Type].Add(cd)
			err := s.st.Next(s.Broker, s.containers[cd.Type])
			if err != nil {
				s.log.Error(err)
				return
			}
		case tk := <-tick:
			_ = tk
			//con := container.NewDataContainer(container.Info{
			//	tk.Code,
			//	time.Second,
			//}, container.Candle{Code: tk.Code})
			//s.st.Next(s.Broker, con)
		}
	}
}

func (s *Engine) Listen(e interface{}) {
	switch et := e.(type) {
	case *order.Order:
		s.st.NotifyOrder(et)
	}
}
