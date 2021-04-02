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
package container

import "time"

type Candle struct {
	Code   string    `json:"code" validate:"required"`
	Open   float64   `json:"open" validate:"required"`
	High   float64   `json:"high" validate:"required"`
	Low    float64   `json:"low" validate:"required"`
	Close  float64   `json:"close" validate:"required"`
	Volume float64   `json:"volume" validate:"required"`
	Date   time.Time `json:"date" validate:"required"`
}
