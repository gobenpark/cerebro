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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/order"
)

// hangUntilDeadline is a market.Order stub that blocks until its (bounded) context
// expires and then returns that context's error, simulating an exchange API that
// never acks within the order timeout.
func hangUntilDeadline(ctx context.Context, _ order.Order) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestSubmit_TimeoutLeavesOrderInDoubt verifies an order whose submit call times out
// is held in-doubt — kept open with its cash reserved and still Submitted — rather
// than rejected, so a possibly-live exchange order is never turned into a phantom by
// releasing its reservation.
func TestSubmit_TimeoutLeavesOrderInDoubt(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(20 * time.Millisecond)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(hangUntilDeadline)

	buy := buyLimit("AAA", 10, 100) // value 1000
	must.NoError(bk.Order(context.Background(), buy, false))
	bk.Wait() // let submit run through to its in-doubt resolution

	open := bk.Orders("AAA")
	must.Len(open, 1, "an in-doubt order stays open, not rejected")
	is.Equal(order.Submitted, open[0].Status(), "it remains Submitted — it may well have reached the exchange")
	eqDec(t, 99_000, bk.Available(), "an in-doubt order keeps its cash reserved")
}

// TestSubmit_InDoubtHandlerInvoked verifies the operator hook fires with the order
// (by its stable client id) when a submission ends in-doubt.
func TestSubmit_InDoubtHandlerInvoked(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(20 * time.Millisecond)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(hangUntilDeadline)

	got := make(chan order.Order, 1)
	bk.SetInDoubtHandler(func(o order.Order, _ error) { got <- o })

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("INDOUBT-1")
	must.NoError(bk.Order(context.Background(), buy, false))
	bk.Wait()

	select {
	case o := <-got:
		must.Equal("INDOUBT-1", o.ID(), "the handler receives the order by its client id")
	case <-time.After(time.Second):
		t.Fatal("in-doubt handler was not invoked")
	}
}

// TestSubmit_InDoubtOrderRemainsCancelable verifies an in-doubt order is marked
// submitted, so an operator's cancel takes the direct path to the market (the order
// may be live there) rather than being deferred forever as unknown.
func TestSubmit_InDoubtOrderRemainsCancelable(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(20 * time.Millisecond)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(hangUntilDeadline)
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).Return(nil)

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("INDOUBT-2")
	must.NoError(bk.Scoped("alpha").Order(context.Background(), buy, false))
	bk.Wait()

	// A direct-path cancel calls market.Cancel synchronously; the gomock expectation
	// above asserts it was reached rather than deferred as an unknown order.
	must.NoError(bk.Scoped("alpha").Cancel(context.Background(), "INDOUBT-2"))
}

// TestSubmit_HungAdapterStillTimesOut guards the genuinely stuck case: an adapter that
// ignores context cancellation and never returns must still not pin the submit
// goroutine. The broker time-boxes the call, resolves the submission in-doubt, and
// Wait returns — instead of hanging the whole engine on shutdown.
func TestSubmit_HungAdapterStillTimesOut(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(20 * time.Millisecond)
	release := make(chan struct{})
	// Ignore ctx entirely: a stuck adapter that won't return on cancellation.
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		<-release
		return nil
	})

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false))

	waited := make(chan struct{})
	go func() { bk.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("Wait hung on a stuck adapter despite WithOrderTimeout")
	}

	is.Len(bk.Orders("AAA"), 1, "the in-doubt order stays open")
	eqDec(t, 99_000, bk.Available(), "an in-doubt order keeps its cash reserved")
	close(release) // let the detached adapter call finish so no goroutine lingers
}

// TestSubmit_ParentCancelDuringDetachedCallIsInDoubt guards the shutdown case: when
// the parent context is canceled while a detached Order call is still running, the
// order must be held in-doubt — not rejected. Rejecting would free the reservation
// while the adapter call may still reach the exchange, the very double-exposure the
// timeout path exists to prevent.
func TestSubmit_ParentCancelDuringDetachedCallIsInDoubt(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(time.Second) // long: we cancel the parent, we do not hit the deadline
	entered := make(chan struct{})
	release := make(chan struct{})
	// Signal the detached call is in flight, then ignore cancellation until released.
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(entered)
		<-release
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	must.NoError(bk.Order(ctx, buyLimit("AAA", 10, 100), false))
	<-entered // the detached Order call is provably in flight before we cancel
	cancel()  // shutdown: parent canceled while the detached Order call is still running

	bk.Wait()

	is.Len(bk.Orders("AAA"), 1, "a parent cancel must not reject a possibly-live order")
	eqDec(t, 99_000, bk.Available(), "the reservation stays — the detached call may still submit")
	close(release) // let the detached adapter call finish
}

// TestSubmit_InDoubtForwardsDeferredCancel guards that a cancel deferred while a
// submit was in flight is still forwarded when that submit ends in-doubt (timed out).
// in-doubt is exactly when the order may be live on the exchange, so dropping the
// queued cancel would strand a possibly-working order the caller was told was canceled.
func TestSubmit_InDoubtForwardsDeferredCancel(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(50 * time.Millisecond)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(hangUntilDeadline)
	canceled := make(chan struct{})
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(canceled)
		return nil
	})
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	buy.SetID("INDOUBT-3")
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	// submit is blocked in market.Order until its deadline, so the order is not yet
	// "submitted"; this cancel must defer rather than reach the market directly.
	must.NoError(bk.Scoped("alpha").Cancel(ctx, "INDOUBT-3"))

	bk.Wait() // submit times out → in-doubt → it must forward the deferred cancel

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("deferred cancel was not forwarded after the in-doubt submission")
	}
}

// TestSubmit_AlreadyCanceledContextRejects verifies an order whose submit runs after
// the run is already canceled is rejected (its reservation freed), not held in-doubt:
// it never reached the exchange, so there is no possibly-live order to protect. The
// adapter must not even be asked to submit.
func TestSubmit_AlreadyCanceledContextRejects(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, _ := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(time.Second)
	// No Order() expectation: a submit on a dead context must not reach the adapter.

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the run is already shutting down before this order is submitted

	must.NoError(bk.Order(ctx, buyLimit("AAA", 10, 100), false))
	bk.Wait()

	is.Empty(bk.Orders("AAA"), "a submit on an already-canceled context is rejected, not in-doubt")
	eqDec(t, 100_000, bk.Available(), "the rejected order frees its reservation")
}

// TestSubmit_DeferredCancelForwardedDuringShutdownWithLiveContext guards the case where
// the run is canceled (shutdown) while a submit is in flight and a cancel was already
// deferred: the cancel must still be forwarded to the exchange, and with a NON-canceled
// context — otherwise a ctx-honoring adapter would skip it, stranding a possibly-live
// order even though the caller requested cancellation and the broker cleared the pending
// flag.
func TestSubmit_DeferredCancelForwardedDuringShutdownWithLiveContext(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(time.Second)
	entered := make(chan struct{})
	release := make(chan struct{})
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(entered)
		<-release // ignore cancellation: the call is still "in flight" at shutdown
		return nil
	})
	canceled := make(chan struct{})
	var cancelCtxErr error
	var cancelBounded bool
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).DoAndReturn(func(cctx context.Context, _ order.Order) error {
		cancelCtxErr = cctx.Err()          // must be nil: non-canceled context
		_, cancelBounded = cctx.Deadline() // must be bounded so it cannot hang shutdown
		close(canceled)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	buy := buyLimit("AAA", 10, 100)
	buy.SetID("SD-1")
	must.NoError(bk.Scoped("a").Order(ctx, buy, false))
	<-entered                                        // the submit is in flight
	must.NoError(bk.Scoped("a").Cancel(ctx, "SD-1")) // deferred: not yet submitted
	cancel()                                         // shutdown while in flight
	close(release)                                   // let the detached Order call finish

	bk.Wait()

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("deferred cancel was not forwarded during shutdown")
	}
	must.NoError(cancelCtxErr, "the deferred cancel must be forwarded with a non-canceled context")
	must.True(cancelBounded, "the deferred cancel must be forwarded with a bounded context")
}

// TestSubmit_DeferredCancelDuringShutdownIsBounded proves the forward's bound actually
// unblocks: an adapter Cancel that honors its context (blocks until it ends) must not
// hang Wait during shutdown, because the forward hands it a bounded context whose
// deadline trips even though the run's cancellation was stripped.
func TestSubmit_DeferredCancelDuringShutdownIsBounded(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(20 * time.Millisecond)
	entered := make(chan struct{})
	releaseOrder := make(chan struct{})
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, order.Order) error {
		close(entered)
		<-releaseOrder
		return nil
	})
	// Honor the forwarded context: block until it ends. With an unbounded WithoutCancel
	// this would never return and Wait would hang; the re-applied bound lets it return.
	mk.EXPECT().Cancel(gomock.Any(), gomock.Any()).DoAndReturn(func(cctx context.Context, _ order.Order) error {
		<-cctx.Done()
		return cctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	buy := buyLimit("AAA", 10, 100)
	buy.SetID("HB-1")
	must.NoError(bk.Scoped("a").Order(ctx, buy, false))
	<-entered
	must.NoError(bk.Scoped("a").Cancel(ctx, "HB-1"))
	cancel()
	close(releaseOrder)

	waited := make(chan struct{})
	go func() { bk.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait hung on a deferred cancel during shutdown despite the bound")
	}
}

// TestSubmit_OutrightErrorStillRejectsWithTimeout verifies a fast, definitive failure
// is still a rejection even with an order timeout set: the call failed before the
// deadline, so the exchange did not get the order and its reservation must be freed.
func TestSubmit_OutrightErrorStillRejectsWithTimeout(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	bk.SetOrderTimeout(time.Second) // generous: the call fails fast, not by timeout
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(errors.New("exchange rejected"))

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false))
	bk.Wait()

	is.Empty(bk.Orders("AAA"), "an outright failure is rejected, not held in-doubt")
	eqDec(t, 100_000, bk.Available(), "a rejected order releases its reservation")
}
