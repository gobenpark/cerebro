package broker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	eventmock "github.com/gobenpark/cerebro/event/mock"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

type noopLogger struct{}

func (noopLogger) Error(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Panic(string, ...any) {}

// newBrokerUnderTest builds a broker seeded with balance. Account reads are
// wired AnyTimes so each test only asserts on the behavior it cares about; the
// returned mock lets a test add an Order() expectation when it submits.
func newBrokerUnderTest(t *testing.T, balance int64, commission float64) (*broker.Broker, *marketmock.MockMarket) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions().Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance().Return(balance).AnyTimes()
	mk.EXPECT().Commission().Return(commission).AnyTimes()

	bc := eventmock.NewMockBroadcaster(ctrl)
	bc.EXPECT().BroadCast(gomock.Any()).AnyTimes()

	return broker.NewDefaultBroker(bc, mk, noopLogger{}), mk
}

func buyLimit(code string, size, price int64) order.Order {
	return order.NewOrder(&item.Item{Code: code}, order.Buy, order.Limit, size, price)
}

func TestNewDefaultBroker_SeedsBalanceFromMarket(t *testing.T) {
	is := assert.New(t)

	bk, _ := newBrokerUnderTest(t, 100_000, 0)

	is.Equal(int64(100_000), bk.Balance())
	is.Equal(int64(100_000), bk.Available())
}

func TestOrder_BuyReservesCash(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false)) // value 1000

	is.Equal(int64(99_000), bk.Available(), "available = balance - reserved")
	is.Equal(int64(100_000), bk.Balance(), "settled balance is unchanged until the exchange settles")
}

func TestOrder_RejectsWhenOverCommitted(t *testing.T) {
	is := assert.New(t)

	bk, _ := newBrokerUnderTest(t, 500, 0)

	err := bk.Order(context.Background(), buyLimit("AAA", 10, 100), false) // value 1000 > 500

	is.ErrorIs(err, broker.ErrNotEnoughMoney)
	is.Equal(int64(500), bk.Available(), "a rejected order must not reserve cash")
}

func TestOrder_ReservationAccumulatesAcrossOpenOrders(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 2_500, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false))
	is.Equal(int64(1_500), bk.Available())

	must.NoError(bk.Order(context.Background(), buyLimit("BBB", 10, 100), false))
	is.Equal(int64(500), bk.Available())

	// Third order needs 1000 but only 500 is available.
	is.ErrorIs(bk.Order(context.Background(), buyLimit("CCC", 10, 100), false), broker.ErrNotEnoughMoney)
}

func TestSubmit_RejectionReleasesReservation(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(errors.New("exchange rejected")).AnyTimes()

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false))

	// submit() runs in a goroutine; rejection asynchronously releases the reservation.
	is.Eventually(func() bool {
		return bk.Available() == 100_000
	}, time.Second, 10*time.Millisecond, "a rejected order should release its reservation")
}

func TestListen_ChangeBalanceEventUpdatesBalance(t *testing.T) {
	is := assert.New(t)

	bk, _ := newBrokerUnderTest(t, 100_000, 0)

	bk.Listen(context.Background(), market.ChangeBalanceEvent{Message: "settled", Balance: 50_000})

	is.Equal(int64(50_000), bk.Balance())
	is.Equal(int64(50_000), bk.Available())
}

// TestListen_TerminalOrderStatusReleasesReservation verifies that every terminal
// exchange status — not just Completed/Canceled — frees the order's reservation.
// A missing case would keep a dead order reserving cash and starve later orders.
func TestListen_TerminalOrderStatusReleasesReservation(t *testing.T) {
	terminal := map[string]order.Status{
		"completed": order.Completed,
		"canceled":  order.Canceled,
		"expired":   order.Expired,
		"margin":    order.Margin,
		"rejected":  order.Rejected,
	}

	for name, status := range terminal {
		t.Run(name, func(t *testing.T) {
			is := assert.New(t)
			must := require.New(t)

			bk, mk := newBrokerUnderTest(t, 100_000, 0)

			// Signal once submit() has handed the order to the market, so the
			// status change below does not run while submit still touches it.
			submitted := make(chan struct{})
			mk.EXPECT().
				Order(gomock.Any(), gomock.Any()).
				Return(nil).
				Do(func(context.Context, order.Order) { close(submitted) })

			o := buyLimit("AAA", 10, 100)
			must.NoError(bk.Order(context.Background(), o, false))
			must.Equal(int64(99_000), bk.Available())
			<-submitted

			bk.Listen(context.Background(), market.ChangeOrderEvent{ID: o.ID(), Action: status})

			is.Equal(int64(100_000), bk.Available(), "terminal status must release the reservation")
		})
	}
}

// TestListen_AcceptedOrderKeepsReservation guards the inverse: a non-terminal
// status leaves the order open and its cash still reserved.
func TestListen_AcceptedOrderKeepsReservation(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	submitted := make(chan struct{})
	mk.EXPECT().
		Order(gomock.Any(), gomock.Any()).
		Return(nil).
		Do(func(context.Context, order.Order) { close(submitted) })

	o := buyLimit("AAA", 10, 100)
	must.NoError(bk.Order(context.Background(), o, false))
	<-submitted

	bk.Listen(context.Background(), market.ChangeOrderEvent{ID: o.ID(), Action: order.Accepted})

	is.Equal(int64(99_000), bk.Available(), "accepted (non-terminal) order keeps its reservation")
}
