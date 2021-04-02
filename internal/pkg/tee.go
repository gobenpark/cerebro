/*
 *                     GNU GENERAL PUBLIC LICENSE
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

package pkg

import (
	"context"

	"github.com/gobenpark/trader/container"
)

func Tee(ctx context.Context, in <-chan container.Tick) (_, _ <-chan container.Tick) {
	out1 := make(chan container.Tick)
	out2 := make(chan container.Tick)

	go func() {
		defer close(out1)
		defer close(out2)

		for val := range OrDone(ctx, in) {
			var out1, out2 = out1, out2
			for i := 0; i < 2; i++ {
				select {
				case <-ctx.Done():
				case out1 <- val:
					out1 = nil
				case out2 <- val:
					out2 = nil
				}
			}
		}
	}()
	return out1, out2
}
