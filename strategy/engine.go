/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
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
		for i := range data {
			for _, strategy := range s.Sts {
				strategy.Next(s.Broker, i)
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
