/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
 */
package strategy

//go:generate mockgen -source=./strategy.go -destination=./mock/mock_strategy.go

import (
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
)

type Strategy interface {
	Next(broker *broker.Broker, container container.Container)

	NotifyOrder(o *order.Order)
	NotifyTrade()
	NotifyCashValue()
	NotifyFund()
}
