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
