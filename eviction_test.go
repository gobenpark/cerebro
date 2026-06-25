/*
 *  Copyright 2021 The Cerebro Authors
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
package cerebro

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// captureSubmitter records the orders an eviction policy places.
type captureSubmitter struct{ orders []order.Order }

func (c *captureSubmitter) Order(_ context.Context, o order.Order, _ bool) error {
	c.orders = append(c.orders, o)
	return nil
}
func (c *captureSubmitter) Available() decimal.Decimal { return decimal.Zero }
func (c *captureSubmitter) Balance() decimal.Decimal   { return decimal.Zero }
func (c *captureSubmitter) Position(string) (position.Position, bool) {
	return position.Position{}, false
}
func (c *captureSubmitter) Orders(string) []order.Order { return nil }

func eviction(size int64, pending bool) Eviction {
	it := &item.Item{Code: "AAA"}
	return Eviction{
		Strategy: "scan:AAA",
		Item:     it,
		Position: position.Position{Item: it, Size: decimal.NewFromInt(size)},
		Pending:  pending,
	}
}

func TestKeepUntilFlat(t *testing.T) {
	is := assert.New(t)
	is.True(KeepUntilFlat(context.Background(), eviction(0, false), nil), "settled (flat, no pending) -> evict")
	is.False(KeepUntilFlat(context.Background(), eviction(5, false), nil), "holding -> keep")
	is.False(KeepUntilFlat(context.Background(), eviction(0, true), nil), "flat but order pending -> keep")
}

func TestFlatten(t *testing.T) {
	is := assert.New(t)

	sub := &captureSubmitter{}
	is.False(Flatten(context.Background(), eviction(5, false), sub), "holding -> submit exit and keep")
	is.Len(sub.orders, 1)
	is.Equal(order.Sell, sub.orders[0].Action())
	is.Equal(order.Market, sub.orders[0].Type())
	is.True(sub.orders[0].Size().Equal(decimal.NewFromInt(5)))

	pend := &captureSubmitter{}
	is.False(Flatten(context.Background(), eviction(0, true), pend), "flat but pending -> keep, no new order")
	is.Empty(pend.orders)

	heldPend := &captureSubmitter{}
	is.False(Flatten(context.Background(), eviction(5, true), heldPend), "holding with an exit already pending -> keep, no duplicate exit")
	is.Empty(heldPend.orders)

	done := &captureSubmitter{}
	is.True(Flatten(context.Background(), eviction(0, false), done), "settled -> evict")
	is.Empty(done.orders)
}

func TestDropImmediately(t *testing.T) {
	is := assert.New(t)
	is.True(DropImmediately(context.Background(), eviction(5, true), nil), "always evict, even holding/pending")
}
