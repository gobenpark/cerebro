package broker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
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
	"github.com/gobenpark/cerebro/risk"
)

type noopLogger struct{}

func (noopLogger) Error(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Panic(string, ...any) {}

// eqDec asserts got equals the integer amount want, comparing numerically so a
// different decimal scale introduced by arithmetic does not fail the check.
func eqDec(t *testing.T, want int64, got decimal.Decimal, msgAndArgs ...any) {
	t.Helper()
	assert.Truef(t, decimal.NewFromInt(want).Equal(got), "want %d, got %s %v", want, got.String(), msgAndArgs)
}

// newBrokerUnderTest builds a broker seeded with balance. Account reads are
// wired AnyTimes so each test only asserts on the behavior it cares about; the
// returned mock lets a test add an Order() expectation when it submits.
func newBrokerUnderTest(t *testing.T, balance int64, commission float64) (*broker.Broker, *marketmock.MockMarket) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountPositions().Return([]position.Position{}).AnyTimes()
	mk.EXPECT().AccountBalance().Return(decimal.NewFromInt(balance)).AnyTimes()
	mk.EXPECT().Commission().Return(decimal.NewFromFloat(commission)).AnyTimes()

	bc := eventmock.NewMockBroadcaster(ctrl)
	bc.EXPECT().BroadCast(gomock.Any()).AnyTimes()
	bc.EXPECT().BroadCastContext(gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	return broker.NewDefaultBroker(bc, mk, noopLogger{}), mk
}

func buyLimit(code string, size, price int64) order.Order {
	return order.NewOrder(&item.Item{Code: code}, order.Buy, order.Limit, decimal.NewFromInt(size), decimal.NewFromInt(price))
}

func TestNewDefaultBroker_SeedsBalanceFromMarket(t *testing.T) {
	bk, _ := newBrokerUnderTest(t, 100_000, 0)

	eqDec(t, 100_000, bk.Balance())
	eqDec(t, 100_000, bk.Available())
}

func TestOrder_BuyReservesCash(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false)) // value 1000

	eqDec(t, 99_000, bk.Available(), "available = balance - reserved")
	eqDec(t, 100_000, bk.Balance(), "settled balance is unchanged until the exchange settles")
}

func TestOrder_RejectsWhenOverCommitted(t *testing.T) {
	is := assert.New(t)

	bk, _ := newBrokerUnderTest(t, 500, 0)

	err := bk.Order(context.Background(), buyLimit("AAA", 10, 100), false) // value 1000 > 500

	is.ErrorIs(err, broker.ErrNotEnoughMoney)
	eqDec(t, 500, bk.Available(), "a rejected order must not reserve cash")
}

func TestOrder_ReservationAccumulatesAcrossOpenOrders(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 2_500, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	must.NoError(bk.Order(context.Background(), buyLimit("AAA", 10, 100), false))
	eqDec(t, 1_500, bk.Available())

	must.NoError(bk.Order(context.Background(), buyLimit("BBB", 10, 100), false))
	eqDec(t, 500, bk.Available())

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
		return bk.Available().Equal(decimal.NewFromInt(100_000))
	}, time.Second, 10*time.Millisecond, "a rejected order should release its reservation")
}

func TestListen_ChangeBalanceEventUpdatesBalance(t *testing.T) {
	bk, _ := newBrokerUnderTest(t, 100_000, 0)

	bk.Listen(context.Background(), market.ChangeBalanceEvent{Message: "settled", Balance: decimal.NewFromInt(50_000)})

	eqDec(t, 50_000, bk.Balance())
	eqDec(t, 50_000, bk.Available())
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
			eqDec(t, 99_000, bk.Available())
			<-submitted

			bk.Listen(context.Background(), market.ChangeOrderEvent{ID: o.ID(), Action: status})

			eqDec(t, 100_000, bk.Available(), "terminal status must release the reservation")
		})
	}
}

// TestListen_AcceptedOrderKeepsReservation guards the inverse: a non-terminal
// status leaves the order open and its cash still reserved.
func TestListen_AcceptedOrderKeepsReservation(t *testing.T) {
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

	eqDec(t, 99_000, bk.Available(), "accepted (non-terminal) order keeps its reservation")
}

// TestListen_CompletedFillRefreshesPositions verifies a fill is reflected in the
// broker's positions when the order completes, so a risk snapshot taken right
// after the fill notification sees the new exposure rather than missing it.
func TestListen_CompletedFillRefreshesPositions(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().AccountBalance().Return(decimal.NewFromInt(100_000)).AnyTimes()
	mk.EXPECT().Commission().Return(decimal.Zero).AnyTimes()
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// The exchange reports the position once the order has filled.
	mk.EXPECT().AccountPositions().Return([]position.Position{
		{Item: &item.Item{Code: "AAA"}, Size: decimal.NewFromInt(10), Price: decimal.NewFromInt(100)},
	}).AnyTimes()

	bc := eventmock.NewMockBroadcaster(ctrl)
	bc.EXPECT().BroadCast(gomock.Any()).AnyTimes()
	bc.EXPECT().BroadCastContext(gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	bk := broker.NewDefaultBroker(bc, mk, noopLogger{})

	o := buyLimit("AAA", 10, 100)
	must.NoError(bk.Order(context.Background(), o, false))
	bk.Listen(context.Background(), market.ChangeOrderEvent{ID: o.ID(), Action: order.Completed})

	pos, ok := bk.Position("AAA")
	must.True(ok)
	is.True(decimal.NewFromInt(10).Equal(pos.Size), "completed fill must be reflected in positions")
}

// TestScoped_StampsStrategyOnOrder verifies a scoped broker tags every order it
// submits with the strategy's name, so fills can be attributed back to it.
func TestScoped_StampsStrategyOnOrder(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	o := buyLimit("AAA", 1, 100)
	must.NoError(bk.Scoped("alpha").Order(context.Background(), o, false))

	is.Equal("alpha", o.Strategy())
}

// TestOrder_RiskGateRejectsBeforeReserving verifies the pre-trade risk gate
// rejects a violating order before any cash is reserved or it is submitted.
func TestOrder_RiskGateRejectsBeforeReserving(t *testing.T) {
	is := assert.New(t)

	bk, _ := newBrokerUnderTest(t, 100_000, 0)
	bk.SetRisk(risk.New(risk.MaxOrderValue(decimal.NewFromInt(500))))

	err := bk.Order(context.Background(), buyLimit("AAA", 10, 100), false) // value 1000 > 500

	is.ErrorIs(err, risk.ErrOrderTooBig)
	eqDec(t, 100_000, bk.Available(), "a risk-rejected order reserves no cash")
}

// TestListen_PartialFillReducesReservation verifies a partial fill keeps the order
// open and shrinks its cash reservation to the unfilled remainder.
func TestListen_PartialFillReducesReservation(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	submitted := make(chan struct{})
	mk.EXPECT().
		Order(gomock.Any(), gomock.Any()).
		Return(nil).
		Do(func(context.Context, order.Order) { close(submitted) })

	o := buyLimit("AAA", 10, 100) // value 1000
	must.NoError(bk.Order(context.Background(), o, false))
	eqDec(t, 99_000, bk.Available())
	<-submitted

	// 4 of 10 fill: remaining 6 @ 100 -> reservation drops from 1000 to 600.
	bk.Listen(context.Background(), market.ChangeOrderEvent{ID: o.ID(), Action: order.Partial, FilledSize: decimal.NewFromInt(4)})

	eqDec(t, 99_400, bk.Available(), "partial fill releases the filled portion's reservation")
	is.Len(bk.Orders("AAA"), 1, "a partially filled order stays open")
}
