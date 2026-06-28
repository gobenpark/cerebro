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

package broker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
)

// TestBroker_CancelReleasesReservation verifies a scoped Cancel asks the market to
// cancel and that the buy's cash reservation is released once the exchange confirms
// with a Canceled event — the same event-driven release path a live fill uses.
func TestBroker_CancelReleasesReservation(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).Return(nil)
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Wait() // let the async submit reach the market so the cancel takes the direct path
	eqDec(t, 99_000, bk.Available(), "the open buy reserves 1000")

	must.NoError(bk.Scoped("alpha").Cancel(ctx, buy.ID()))
	// The exchange confirms the cancellation, which releases the reservation.
	bk.Listen(ctx, market.ChangeOrderEvent{ID: buy.ID(), Action: order.Canceled, Message: "canceled"})

	eqDec(t, 100_000, bk.Available(), "reservation released on cancel confirmation")
	is.Empty(bk.Orders("AAA"), "the canceled order leaves the open set")
}

// TestBroker_CancelRejectsForeignAndUnknown verifies a strategy can only cancel its
// own open orders: another strategy's order and an unknown id are both rejected
// without ever asking the market to cancel.
func TestBroker_CancelRejectsForeignAndUnknown(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// No Cancel expectation: the market must NOT be asked to cancel in either case.
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))

	err := bk.Scoped("beta").Cancel(ctx, buy.ID())
	must.ErrorContains(err, "not owned", "a strategy cannot cancel another's order")

	err = bk.Scoped("alpha").Cancel(ctx, "no-such-id")
	must.ErrorContains(err, "no open order")
}

// TestBroker_CancelBeforeSubmitIsForwarded guards the race where a cancel arrives
// before the async submit has told the market about the order. Holding market.Order
// until after the cancel forces that interleaving: the cancel must be deferred and
// then forwarded once submission completes, not dropped as an unknown cancel.
func TestBroker_CancelBeforeSubmitIsForwarded(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	proceed := make(chan struct{})
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		<-proceed // block submit inside market.Order until the test has issued the cancel
		return nil
	})
	canceled := make(chan struct{})
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(canceled)
		return nil
	})
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	// submit is blocked in market.Order, so the order is not yet "submitted"; the
	// cancel must defer rather than race ahead of the order.
	must.NoError(bk.Scoped("alpha").Cancel(ctx, buy.ID()))

	close(proceed) // let submit finish — it must forward the deferred cancel
	bk.Wait()

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("deferred cancel was not forwarded to the market after submission")
	}
}

// TestBroker_SubmitDoesNotRemarkTerminalOrder guards the race where a terminal event
// is processed while submit is still inside market.Order: submit must not re-add the
// already-cleared id to the submitted set, or a later order reusing the id would take
// the wrong (direct) cancel path.
func TestBroker_SubmitDoesNotRemarkTerminalOrder(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	ctx := context.Background()

	// First order id "X": hold its submit inside market.Order, complete it via a
	// terminal event during that window, then release. submit must not re-mark "X".
	proceed := make(chan struct{})
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		<-proceed
		return nil
	})
	first := buyLimit("AAA", 10, 100)
	first.SetID("X")
	must.NoError(bk.Scoped("alpha").Order(ctx, first, false))
	bk.Listen(ctx, completedFill(first, 100, 10)) // terminal, while submit is blocked
	close(proceed)
	bk.Wait()

	// Reuse id "X": block submit and cancel before it registers. A stale submitted
	// "X" would make the cancel take the direct path; with the fix it defers.
	proceed2 := make(chan struct{})
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		<-proceed2
		return nil
	})
	canceled := make(chan struct{})
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(canceled)
		return nil
	})
	second := buyLimit("AAA", 10, 100)
	second.SetID("X")
	must.NoError(bk.Scoped("alpha").Order(ctx, second, false))
	must.NoError(bk.Scoped("alpha").Cancel(ctx, "X"))

	select {
	case <-canceled:
		t.Fatal("cancel took the direct path: submit re-marked a terminal id as submitted")
	default:
	}

	close(proceed2)
	bk.Wait()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("deferred cancel was not forwarded after submission")
	}
}

// TestBroker_TerminalEventClearsSubmittedID guards that a terminal exchange event
// clears the order's id from the submit bookkeeping, so a later order reusing the id
// is not mistaken for already-submitted. Without the cleanup the reused order's
// cancel-before-submit would wrongly take the direct path and be lost.
func TestBroker_TerminalEventClearsSubmittedID(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	ctx := context.Background()

	// First order with a fixed id: submit it, then complete it via a terminal event,
	// which must clear "REUSE" from the submitted set.
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	first := buyLimit("AAA", 10, 100)
	first.SetID("REUSE")
	must.NoError(bk.Scoped("alpha").Order(ctx, first, false))
	bk.Wait()
	bk.Listen(ctx, completedFill(first, 100, 10))

	// A second order reuses the id; block its submit and cancel before it registers.
	proceed := make(chan struct{})
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		<-proceed
		return nil
	})
	canceled := make(chan struct{})
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(canceled)
		return nil
	})

	second := buyLimit("AAA", 10, 100)
	second.SetID("REUSE")
	must.NoError(bk.Scoped("alpha").Order(ctx, second, false))
	must.NoError(bk.Scoped("alpha").Cancel(ctx, "REUSE"))

	// A direct-path cancel would have called market.Cancel synchronously already; with
	// the id cleared, the cancel deferred and nothing has been forwarded yet.
	select {
	case <-canceled:
		t.Fatal("cancel took the direct path: a stale submitted id was not cleared on the terminal event")
	default:
	}

	close(proceed)
	bk.Wait()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("deferred cancel was not forwarded after submission")
	}
}
