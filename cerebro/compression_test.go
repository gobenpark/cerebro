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

package cerebro

import (
	"testing"
	"time"

	"github.com/gobenpark/trader/container"
	"github.com/stretchr/testify/assert"
)

func TestCompression(t *testing.T) {
	ti1, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:00:01")
	ti2, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:01:01")
	ti3, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:01:02")
	ti4, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:02:02")
	ti5, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:03:02")
	ti6, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:04:02")
	ti7, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:05:02")
	ti8, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:06:02")
	ti9, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:07:02")
	input := []container.Tick{
		{
			Code:   "test",
			Date:   ti1,
			Price:  1,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti2,
			Price:  2,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti3,
			Price:  3,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti4,
			Price:  4,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti5,
			Price:  10,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti6,
			Price:  5,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti7,
			Price:  6,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti8,
			Price:  6,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti9,
			Price:  6,
			Volume: 10,
		},
	}
	ch := make(chan container.Tick)
	go func() {
		defer close(ch)
		for _, i := range input {
			ch <- i
		}
	}()

	rightedge := []container.Candle{}
	for d := range Compression(ch, time.Minute*3, false) {
		rightedge = append(rightedge, d)
	}
	assert.Len(t, rightedge, 2)

}
