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
package strategy

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

import (
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

type CandleType int

type Strategy interface {
	Next(indicator indicator.Value, b *broker.Broker)
	// Filter is when pass or not for strategy if true then pass else not pass
	Filter(itm item.Item, c CandleProvider) bool
	//NotifyOrder is when event rise order then called
	NotifyOrder(o order.Order)
	NotifyTrade()
	NotifyCashValue(before, after int64)
	NotifyFund()
	Name() string
}
