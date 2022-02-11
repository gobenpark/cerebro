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

package container

import (
	"testing"
	"time"

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
	input := []Tick{
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
	ch := make(chan Tick)
	go func() {
		defer close(ch)
		for _, i := range input {
			ch <- i
		}
	}()

	rightedge := []Candle{}
	for d := range Compression(ch, time.Minute*3, false) {
		rightedge = append(rightedge, d)
	}
	assert.Len(t, rightedge, 2)

}
