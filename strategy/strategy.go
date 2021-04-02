/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
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
