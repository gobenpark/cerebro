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

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

import (
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
)

type CandleType int

const (
	Min1 CandleType = iota + 1
	Min3
	Min5
	Min15
	Min60
	Day
)

type Strategy interface {
	CandleType() CandleType
	Next(broker broker.Broker, container container.Container2) error

	//NotifyOrder is when event rise order then called
	NotifyOrder(o order.Order)
	NotifyTrade()
	NotifyCashValue(before, after int64)
	NotifyFund()
}
