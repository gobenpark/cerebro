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
package indicators

import "github.com/gobenpark/trader/container"

func average(candle []container.Candle) float64 {
	total := 0.0
	for _, v := range candle {
		total += v.Close
	}
	return total / float64(len(candle))
}
