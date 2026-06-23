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
	"github.com/gobenpark/cerebro/indicator"
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

func sellLimit(code string, size, price int64) order.Order {
	return order.NewOrder(&item.Item{Code: code}, order.Sell, order.Limit, decimal.NewFromInt(size), decimal.NewFromInt(price))
}

// completedFill is the exchange event for a full fill of o at the given price.
func completedFill(o order.Order, price, size int64) market.ChangeOrderEvent {
	return market.ChangeOrderEvent{
		ID: o.ID(), Action: order.Completed,
		Price: decimal.NewFromInt(price), FilledSize: decimal.NewFromInt(size),
	}
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

// TestPnL_RealizedOnClose verifies a round trip books realized PnL only when the
// position closes, attributed to the strategy that traded it.
func TestPnL_RealizedOnClose(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10))
	eqDec(t, 0, bk.RealizedPnL("alpha"), "an open position has no realized PnL")

	sell := sellLimit("AAA", 10, 120)
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 120, 10))
	eqDec(t, 200, bk.RealizedPnL("alpha"), "(120-100)*10")
	eqDec(t, 200, bk.TotalRealizedPnL())
}

// TestPnL_ReportTracksFeesAndOpenPosition checks commission accrual and that a
// partial close leaves the average entry of the remaining position unchanged.
func TestPnL_ReportTracksFeesAndOpenPosition(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 1) // 1% commission
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100) // fee 1% of 1000 = 10
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10))

	sell := sellLimit("AAA", 4, 120) // fee 1% of 480 = 4.8; realized (120-100)*4 = 80
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 120, 4))

	rep := bk.Report()
	must.Len(rep, 1)
	is.Equal("alpha", rep[0].Strategy)
	eqDec(t, 80, rep[0].Realized, "(120-100)*4")
	is.True(decimal.NewFromFloat(14.8).Equal(rep[0].Fees), "buy fee 10 + sell fee 4.8")
	must.Len(rep[0].Positions, 1, "6 units remain open")
	eqDec(t, 6, rep[0].Positions[0].Size)
	eqDec(t, 100, rep[0].Positions[0].Price, "a partial close does not move the average entry")
}

// TestPnL_MarketFillWithoutPriceUsesLastTick guards that a market fill from an
// adapter that reports no fill price is still tracked, valued at the last tick —
// so reactive risk policies keep seeing market-entered positions.
func TestPnL_MarketFillWithoutPriceUsesLastTick(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	// A tick establishes the last price for AAA.
	bk.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(100)})

	buy := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Market, decimal.NewFromInt(10), decimal.Zero)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	// The completed event carries no Price (a bare adapter).
	bk.Listen(ctx, market.ChangeOrderEvent{ID: buy.ID(), Action: order.Completed, FilledSize: decimal.NewFromInt(10)})

	pos, _, ok := bk.StrategyPosition("alpha", "AAA")
	must.True(ok, "a market fill must be tracked even without a reported price")
	eqDec(t, 10, pos.Size)
	eqDec(t, 100, pos.Price, "valued at the last tick price")
}

// TestStrategyPosition_TracksHighWaterFillPrice verifies the lot's reported peak is
// the highest fill price across scale-ins, not the average entry.
func TestStrategyPosition_TracksHighWaterFillPrice(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 1_000_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	low := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, low, false))
	bk.Listen(ctx, completedFill(low, 100, 10))

	high := buyLimit("AAA", 10, 200)
	must.NoError(bk.Scoped("alpha").Order(ctx, high, false))
	bk.Listen(ctx, completedFill(high, 200, 10))

	pos, peak, ok := bk.StrategyPosition("alpha", "AAA")
	must.True(ok)
	eqDec(t, 150, pos.Price, "average entry of two equal-size fills at 100 and 200")
	eqDec(t, 200, peak, "high-water fill price is the higher fill, not the average")
}

// TestPnL_MarketFillUsesEventPrice guards that a market order — which carries no
// price of its own — is valued at the exchange's reported fill price.
func TestPnL_MarketFillUsesEventPrice(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	buy := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Market, decimal.NewFromInt(10), decimal.Zero)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10)) // fills at 100

	sell := order.NewOrder(&item.Item{Code: "AAA"}, order.Sell, order.Market, decimal.NewFromInt(10), decimal.Zero)
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 130, 10)) // fills at 130

	eqDec(t, 300, bk.RealizedPnL("alpha"), "(130-100)*10 from the event fill prices")
}

// TestPnL_StrategiesAreIsolated verifies one strategy's close does not touch
// another strategy's position in the same code.
func TestPnL_StrategiesAreIsolated(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 1_000_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	a := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, a, false))
	bk.Listen(ctx, completedFill(a, 100, 10))

	bb := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("beta").Order(ctx, bb, false))
	bk.Listen(ctx, completedFill(bb, 100, 10))

	s := sellLimit("AAA", 10, 120)
	must.NoError(bk.Scoped("alpha").Order(ctx, s, false))
	bk.Listen(ctx, completedFill(s, 120, 10))

	eqDec(t, 200, bk.RealizedPnL("alpha"))
	eqDec(t, 0, bk.RealizedPnL("beta"), "beta still holds; nothing realized")
}

// TestPnL_PartialThenCompleteCountsFillOnce verifies a partial fill followed by a
// completion accrues the full acquired size exactly once.
func TestPnL_PartialThenCompleteCountsFillOnce(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, market.ChangeOrderEvent{
		ID: buy.ID(), Action: order.Partial,
		Price: decimal.NewFromInt(100), FilledSize: decimal.NewFromInt(4),
	})
	bk.Listen(ctx, completedFill(buy, 100, 6)) // the remaining 6 fill

	sell := sellLimit("AAA", 10, 120)
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 120, 10))

	eqDec(t, 200, bk.RealizedPnL("alpha"), "10 acquired (4+6), each closed at +20")
}

// TestPnL_OversellClampsToHeld guards the defensive clamp: a reported sell larger
// than the held quantity realizes PnL on only what is actually held.
func TestPnL_OversellClampsToHeld(t *testing.T) {
	must := require.New(t)

	bk, mk := newBrokerUnderTest(t, 100_000, 0)
	mk.EXPECT().Order(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ctx := context.Background()

	buy := buyLimit("AAA", 10, 100)
	must.NoError(bk.Scoped("alpha").Order(ctx, buy, false))
	bk.Listen(ctx, completedFill(buy, 100, 10))

	sell := sellLimit("AAA", 15, 120)
	must.NoError(bk.Scoped("alpha").Order(ctx, sell, false))
	bk.Listen(ctx, completedFill(sell, 120, 15))

	eqDec(t, 200, bk.RealizedPnL("alpha"), "(120-100)*10; the 5-unit oversell is ignored")
	must.Empty(bk.Report()[0].Positions, "position is flat")
}
