package order_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

// newOrder builds a test order on a fixed item with int-valued size/price.
func newOrder(action order.Action, otype order.OrderType, size, price int64) order.Order {
	return order.NewOrder(&item.Item{Code: "AAA"}, action, otype, decimal.NewFromInt(size), decimal.NewFromInt(price))
}

// decEq asserts two decimals are numerically equal via decimal.Equal — not testify's
// DeepEqual, which a differing internal exponent (e.g. from Mul) can fool.
func decEq(t *testing.T, want, got decimal.Decimal) {
	t.Helper()
	assert.Truef(t, want.Equal(got), "want %s, got %s", want, got)
}

// TestOrder_CopyPreservesStrategy guards that Copy carries the strategy tag, so
// the order-update copies the broker broadcasts on fills keep their attribution.
func TestOrder_CopyPreservesStrategy(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, decimal.NewFromInt(1), decimal.NewFromInt(100))
	o.SetStrategy("alpha")

	is.Equal("alpha", o.Copy().Strategy())
}

// TestNewOrder_InitialState verifies a freshly created order is Created, carries its
// inputs, and starts fully unfilled (remaining == size, remain price == notional).
func TestNewOrder_InitialState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		action order.Action
		otype  order.OrderType
		size   int64
		price  int64
	}{
		{"market buy", order.Buy, order.Market, 10, 100},
		{"limit sell", order.Sell, order.Limit, 5, 2000},
		{"stop single share", order.Sell, order.Stop, 1, 99},
		{"zero price (market)", order.Buy, order.Market, 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			it := &item.Item{Code: "AAA"}
			o := order.NewOrder(it, tt.action, tt.otype, decimal.NewFromInt(tt.size), decimal.NewFromInt(tt.price))

			is.Equal(order.Created, o.Status(), "new order starts Created")
			is.Equal(tt.action, o.Action())
			is.Equal(tt.otype, o.Type())
			is.Same(it, o.Item(), "item pointer preserved")
			is.NotEmpty(o.ID(), "a uuid is assigned")

			decEq(t, decimal.NewFromInt(tt.size), o.Size())
			decEq(t, decimal.NewFromInt(tt.price), o.Price())
			decEq(t, decimal.NewFromInt(tt.size), o.RemainingSize())
			decEq(t, decimal.NewFromInt(tt.size*tt.price), o.OrderPrice())
			decEq(t, decimal.NewFromInt(tt.size*tt.price), o.RemainPrice())
		})
	}
}

// TestOrder_StatusTransitions checks each lifecycle transition sets the expected status
// from a fresh (Created) order.
func TestOrder_StatusTransitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		apply func(order.Order)
		want  order.Status
	}{
		{"submit", func(o order.Order) { o.Submit() }, order.Submitted},
		{"accept", func(o order.Order) { o.Accept() }, order.Accepted},
		{"cancel", func(o order.Order) { o.Cancel() }, order.Canceled},
		{"reject", func(o order.Order) { o.Reject() }, order.Rejected},
		{"expire", func(o order.Order) { o.Expire() }, order.Expired},
		{"margin", func(o order.Order) { o.Margin() }, order.Margin},
		{"complete", func(o order.Order) { o.Complete() }, order.Completed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			o := newOrder(order.Buy, order.Market, 10, 100)
			is.Equal(order.Created, o.Status())

			tt.apply(o)

			is.Equal(tt.want, o.Status())
		})
	}
}

// TestOrder_Partial_AccumulatesFills verifies a partial fill sets Partial and shrinks
// the remaining size (and remain price) cumulatively, while the original notional
// (OrderPrice) is unchanged.
func TestOrder_Partial_AccumulatesFills(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	o := newOrder(order.Buy, order.Market, 10, 100)

	o.Partial(decimal.NewFromInt(3))
	is.Equal(order.Partial, o.Status())
	decEq(t, decimal.NewFromInt(7), o.RemainingSize())
	decEq(t, decimal.NewFromInt(700), o.RemainPrice())

	o.Partial(decimal.NewFromInt(4))
	is.Equal(order.Partial, o.Status())
	decEq(t, decimal.NewFromInt(3), o.RemainingSize())
	decEq(t, decimal.NewFromInt(300), o.RemainPrice())

	decEq(t, decimal.NewFromInt(1000), o.OrderPrice()) // notional unchanged by partials
}

// TestOrder_Complete_ZeroesRemaining verifies Complete drives the order to Completed and
// zeroes the remaining size/price, whether reached directly or after a partial fill.
func TestOrder_Complete_ZeroesRemaining(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(order.Order)
	}{
		{"from created", func(order.Order) {}},
		{"after partial", func(o order.Order) { o.Partial(decimal.NewFromInt(6)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			o := newOrder(order.Buy, order.Market, 10, 100)
			tt.setup(o)

			o.Complete()

			is.Equal(order.Completed, o.Status())
			is.True(o.RemainingSize().IsZero(), "remaining zeroed")
			is.True(o.RemainPrice().IsZero(), "remain price zeroed")
			decEq(t, decimal.NewFromInt(1000), o.OrderPrice())
		})
	}
}

// TestOrder_SetID replaces the generated uuid with a broker-assigned id.
func TestOrder_SetID(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	o := newOrder(order.Buy, order.Market, 1, 100)

	gen := o.ID()
	is.NotEmpty(gen)

	o.SetID("broker-123")
	is.Equal("broker-123", o.ID())
	is.NotEqual(gen, o.ID())
}

// TestOrder_Copy_IsIndependentSnapshot verifies Copy is a value snapshot: mutating the
// original afterwards does not affect the copy, while identity/inputs are carried.
func TestOrder_Copy_IsIndependentSnapshot(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	o := newOrder(order.Buy, order.Limit, 10, 100)
	o.Submit()

	snap := o.Copy()
	o.Partial(decimal.NewFromInt(4))
	o.Complete()

	is.Equal(order.Submitted, snap.Status(), "snapshot keeps the copy-time status")
	decEq(t, decimal.NewFromInt(10), snap.RemainingSize())
	is.Equal(order.Completed, o.Status(), "original advanced independently")
	is.True(o.RemainingSize().IsZero())

	is.Equal(o.ID(), snap.ID(), "identity carried")
	is.Equal(o.Action(), snap.Action())
	is.Equal(o.Type(), snap.Type())
	decEq(t, o.Price(), snap.Price())
}

// TestOrder_MarshalJSON checks the public JSON shape: item, price, size, createdAt.
func TestOrder_MarshalJSON(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	o := newOrder(order.Buy, order.Limit, 10, 100)

	b, err := json.Marshal(o)
	is.NoError(err)

	var got map[string]any
	is.NoError(json.Unmarshal(b, &got))
	is.Contains(got, "item")
	is.Contains(got, "createdAt")
	is.Equal("100", got["price"], "price serialized as a decimal string")
	is.Equal("10", got["size"], "size serialized as a decimal string")
}

// TestOrder_ConcurrentAccess_NoRace exercises the RWMutex: concurrent transitions and
// reads must be race-free (run with -race). Every goroutine joins, so none leak.
func TestOrder_ConcurrentAccess_NoRace(t *testing.T) {
	t.Parallel()
	o := newOrder(order.Buy, order.Market, 1000, 10)

	writers := []func(){
		func() { o.Submit() },
		func() { o.Accept() },
		func() { o.Partial(decimal.NewFromInt(1)) },
		func() { o.SetStrategy("alpha") },
		func() { o.SetID("x") },
	}
	readers := []func(){
		func() { _ = o.Status() },
		func() { _ = o.RemainingSize() },
		func() { _ = o.OrderPrice() },
		func() { _ = o.RemainPrice() },
		func() { _ = o.ID() },
		func() { _ = o.Strategy() },
		func() { _ = o.Action() },
	}

	var wg sync.WaitGroup
	run := func(fns []func()) {
		for _, fn := range fns {
			wg.Add(1)
			go func(f func()) {
				defer wg.Done()
				for range 200 {
					f()
				}
			}(fn)
		}
	}
	run(writers)
	run(readers)
	wg.Wait()
}
