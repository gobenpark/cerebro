package strategy_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/strategy"
)

// stubStrategy is an inert strategy; its Run blocks until the context is canceled.
type stubStrategy struct{}

func (stubStrategy) Name() string { return "stub" }
func (stubStrategy) Run(ctx context.Context, _ strategy.Universe, _ broker.Submitter) {
	<-ctx.Done()
}
func (stubStrategy) NotifyOrder(order.Order) {}

// TestEngine_ConcurrentSpawnAndListen drives a writer (Spawn) and reader (Listen)
// against the channels map simultaneously. Without the RWMutex this panics with
// "concurrent map read and write" under -race. No runners keeps Spawn trivial (it
// only resets the map), so no goroutines leak.
func TestEngine_ConcurrentSpawnAndListen(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, nil, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		eng.Spawn(ctx)
	}()
	go func() {
		defer wg.Done()
		// Stay under the per-code buffer (100) since nothing drains it.
		for range 50 {
			eng.Listen(ctx, indicator.Tick{Code: "AAA"})
		}
	}()
	wg.Wait()
}

// TestEngine_SubscribeFailureStartsNoRunners guards the rollback path: when the
// market's Subscribe fails, the engine must not leave a strategy Run goroutine or
// registered channel behind. goleak fails if a runner leaked.
func TestEngine_SubscribeFailureStartsNoRunners(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(errors.New("boom")).AnyTimes()

	runners := []strategy.Runner{{Strategy: stubStrategy{}, Items: []*item.Item{{Code: "AAA"}}}}
	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, runners, mk, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn throttles briefly, then Subscribe fails; the Run goroutine must not
	// start, so Wait returns immediately because nothing was launched.
	eng.Spawn(ctx)
	eng.Wait()
}

// recordingStrategy records the orders it is notified about.
type recordingStrategy struct {
	name string
	mu   sync.Mutex
	got  int
}

func (s *recordingStrategy) Name() string { return s.name }
func (s *recordingStrategy) Run(context.Context, strategy.Universe, broker.Submitter) {
}
func (s *recordingStrategy) NotifyOrder(order.Order) { s.mu.Lock(); s.got++; s.mu.Unlock() }
func (s *recordingStrategy) count() int              { s.mu.Lock(); defer s.mu.Unlock(); return s.got }

// TestEngine_NotifyOrderRoutesToOwningStrategy verifies an attributed order is
// delivered only to its owning runner, while an unattributed one goes to all.
func TestEngine_NotifyOrderRoutesToOwningStrategy(t *testing.T) {
	is := assert.New(t)

	a := &recordingStrategy{name: "a"}
	b := &recordingStrategy{name: "b"}
	runners := []strategy.Runner{
		{Strategy: a, Items: []*item.Item{{Code: "AAA"}}},
		{Strategy: b, Items: []*item.Item{{Code: "BBB"}}},
	}
	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, runners, nil, 0)

	mk := func(owner string) order.Order {
		o := order.NewOrder(&item.Item{Code: "AAA"}, order.Buy, order.Limit, decimal.NewFromInt(1), decimal.NewFromInt(100))
		if owner != "" {
			o.SetStrategy(owner)
		}
		return o
	}

	eng.Listen(context.Background(), mk("a"))
	is.Equal(1, a.count(), "owning strategy is notified")
	is.Equal(0, b.count(), "other strategy is not notified")

	eng.Listen(context.Background(), mk("")) // unattributed -> all
	is.Equal(2, a.count())
	is.Equal(1, b.count())
}

// tickRecorder records the distinct codes it receives on its universe channel.
type tickRecorder struct {
	mu    sync.Mutex
	codes map[string]int
}

func (s *tickRecorder) Name() string { return "portfolio" }
func (s *tickRecorder) Run(ctx context.Context, u strategy.Universe, _ broker.Submitter) {
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-u.Ticks():
			if !ok {
				return
			}
			s.mu.Lock()
			s.codes[tk.Code]++
			s.mu.Unlock()
		}
	}
}
func (s *tickRecorder) NotifyOrder(order.Order) {}
func (s *tickRecorder) seen(code string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.codes[code]
}

// TestEngine_PortfolioRunnerReceivesAllCodes is the core multi-asset behavior: a
// single runner over a {AAA,BBB} universe receives ticks for BOTH codes on its one
// channel, so a pairs/portfolio strategy can decide over them together.
func TestEngine_PortfolioRunnerReceivesAllCodes(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	rec := &tickRecorder{codes: map[string]int{}}
	runners := []strategy.Runner{{
		Strategy: rec,
		Items:    []*item.Item{{Code: "AAA"}, {Code: "BBB"}},
	}}
	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, runners, mk, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng.Spawn(ctx) // blocks through the launch throttle; the Run goroutine is live after
	eng.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(10)})
	eng.Listen(ctx, indicator.Tick{Code: "BBB", Price: decimal.NewFromInt(20)})

	is.Eventually(func() bool {
		return rec.seen("AAA") > 0 && rec.seen("BBB") > 0
	}, 2*time.Second, 10*time.Millisecond, "one runner must receive both codes of its universe")

	cancel()
	eng.Wait()
}

// bookRecorder records the distinct codes it receives on its order-book channel.
type bookRecorder struct {
	mu    sync.Mutex
	codes map[string]int
}

func (s *bookRecorder) Name() string { return "book" }
func (s *bookRecorder) Run(ctx context.Context, u strategy.Universe, _ broker.Submitter) {
	for {
		select {
		case <-ctx.Done():
			return
		case ob, ok := <-u.OrderBooks():
			if !ok {
				return
			}
			s.mu.Lock()
			s.codes[ob.Code]++
			s.mu.Unlock()
		}
	}
}
func (s *bookRecorder) NotifyOrder(order.Order) {}
func (s *bookRecorder) seen(code string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.codes[code]
}

// TestEngine_RoutesOrderBooksByCode is the order-book counterpart of the tick
// routing: a runner over {AAA,BBB} receives order-book snapshots for BOTH codes on
// its single OrderBooks() channel, demultiplexed by indicator.OrderBook.Code.
func TestEngine_RoutesOrderBooksByCode(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	rec := &bookRecorder{codes: map[string]int{}}
	runners := []strategy.Runner{{
		Strategy: rec,
		Items:    []*item.Item{{Code: "AAA"}, {Code: "BBB"}},
	}}
	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, runners, mk, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng.Spawn(ctx) // blocks through the launch throttle; the Run goroutine is live after
	eng.Listen(ctx, indicator.OrderBook{
		Code: "AAA",
		Bids: []indicator.Level{{Price: decimal.NewFromInt(99), Size: decimal.NewFromInt(1)}},
	})
	eng.Listen(ctx, indicator.OrderBook{
		Code: "BBB",
		Asks: []indicator.Level{{Price: decimal.NewFromInt(101), Size: decimal.NewFromInt(1)}},
	})

	is.Eventually(func() bool {
		return rec.seen("AAA") > 0 && rec.seen("BBB") > 0
	}, 2*time.Second, 10*time.Millisecond, "one runner must receive order books for both codes of its universe")

	cancel()
	eng.Wait()
}

// TestEngine_AddRunnerThenRemove exercises the dynamic lifecycle: a runner added
// after Spawn receives ticks for its code, a duplicate add is rejected, and
// RemoveRunner both stops its Run goroutine (goleak fails if it leaked) and
// unregisters its channel so later ticks are no longer routed to it.
func TestEngine_AddRunnerThenRemove(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)
	must := require.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, nil, mk, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng.Spawn(ctx) // no initial runners

	rec := &tickRecorder{codes: map[string]int{}}
	r := strategy.Runner{Strategy: rec, Items: []*item.Item{{Code: "AAA"}}}
	must.NoError(eng.AddRunner(ctx, r))

	// The runtime-added runner receives ticks for its code.
	eng.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(10)})
	is.Eventually(func() bool { return rec.seen("AAA") > 0 }, 2*time.Second, 10*time.Millisecond,
		"a runner added after Spawn must receive ticks for its code")

	// A second add under the same name is rejected.
	is.Error(eng.AddRunner(ctx, r), "a runner already running cannot be added again")

	// Remove it: its Run goroutine must exit (goleak verifies) and a later tick must
	// neither panic on the abandoned channel nor reach the removed runner.
	before := rec.seen("AAA")
	eng.RemoveRunner(ctx, rec.Name())
	eng.Listen(ctx, indicator.Tick{Code: "AAA", Price: decimal.NewFromInt(11)})
	is.Never(func() bool { return rec.seen("AAA") > before }, 200*time.Millisecond, 20*time.Millisecond,
		"a removed runner must no longer receive ticks")

	cancel()
	eng.Wait()
}

// progEvent is an adapter-specific Extras event tagged to a code (implements Coded).
type progEvent struct {
	code string
	val  int
}

func (e progEvent) Code() string { return e.code }

// blastEvent is a code-less Extras event — it implements no Coded, so the engine
// broadcasts it to every universe.
type blastEvent struct{ val int }

// fakeUniverse is a minimal Universe whose Extras channel a test drives directly, to
// unit-test Stream[T] in isolation from the engine's fan-out.
type fakeUniverse struct{ extras chan any }

func (f *fakeUniverse) Items() []*item.Item                    { return nil }
func (f *fakeUniverse) Ticks() <-chan indicator.Tick           { return nil }
func (f *fakeUniverse) OrderBooks() <-chan indicator.OrderBook { return nil }
func (f *fakeUniverse) Extras() <-chan any                     { return f.extras }
func (f *fakeUniverse) Warmup(context.Context, string, market.CandleType) (strategy.CandleStream, error) {
	return nil, nil
}

// TestStream_FiltersByTypeAndStopsOnCancel verifies Stream[T] forwards only the Extras
// values whose dynamic type is T (dropping the rest), and that its goroutine exits when
// ctx is canceled (goleak), closing the returned channel.
func TestStream_FiltersByTypeAndStopsOnCancel(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)

	fu := &fakeUniverse{extras: make(chan any, 8)}
	ctx, cancel := context.WithCancel(context.Background())

	out := strategy.Stream[progEvent](ctx, fu)
	fu.extras <- blastEvent{val: 1}             // wrong type → dropped
	fu.extras <- progEvent{code: "AAA", val: 2} // right type → forwarded

	got := <-out
	is.Equal("AAA", got.code, "Stream forwards only T-typed values")
	is.Equal(2, got.val)

	cancel()
	_, ok := <-out
	is.False(ok, "Stream closes its channel when ctx is canceled")
}

// extraRecorder consumes its universe's raw Extras stream with one goroutine (so no two
// Stream[T] consumers compete for the single channel), counting Coded program events by
// code and code-less blast events.
type extraRecorder struct {
	name   string
	mu     sync.Mutex
	byCode map[string]int
	blasts int
	other  int // any Extras value that is neither progEvent nor blastEvent — must stay 0
}

func (s *extraRecorder) Name() string { return s.name }
func (s *extraRecorder) Run(ctx context.Context, u strategy.Universe, _ broker.Submitter) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-u.Extras():
			if !ok {
				return
			}
			s.mu.Lock()
			switch ev := e.(type) {
			case progEvent:
				s.byCode[ev.code]++
			case blastEvent:
				s.blasts++
			default:
				s.other++
			}
			s.mu.Unlock()
		}
	}
}
func (s *extraRecorder) NotifyOrder(order.Order) {}
func (s *extraRecorder) seen(code string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byCode[code]
}
func (s *extraRecorder) blastCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blasts
}
func (s *extraRecorder) otherCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.other
}

// TestEngine_RoutesExtrasByCodeAndBroadcast verifies the engine's Extras fan-out: a
// Coded event reaches only the universe subscribed to its code, while a code-less one
// broadcasts to every universe.
func TestEngine_RoutesExtrasByCodeAndBroadcast(t *testing.T) {
	defer goleak.VerifyNone(t)
	is := assert.New(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	a := &extraRecorder{name: "ea", byCode: map[string]int{}}
	b := &extraRecorder{name: "eb", byCode: map[string]int{}}
	runners := []strategy.Runner{
		{Strategy: a, Items: []*item.Item{{Code: "AAA"}}},
		{Strategy: b, Items: []*item.Item{{Code: "BBB"}}},
	}
	eng := strategy.NewEngine(slog.New(slog.DiscardHandler), nil, runners, mk, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng.Spawn(ctx)

	// A Coded event is delivered only to the universe subscribed to its code.
	eng.Listen(ctx, progEvent{code: "AAA", val: 1})
	is.Eventually(func() bool { return a.seen("AAA") > 0 }, 2*time.Second, 10*time.Millisecond,
		"a Coded extras event reaches the universe subscribed to its code")
	is.Never(func() bool { return b.seen("AAA") > 0 }, 200*time.Millisecond, 20*time.Millisecond,
		"a Coded extras event does not reach other codes' universes")

	// A code-less event broadcasts to every universe.
	eng.Listen(ctx, blastEvent{val: 9})
	is.Eventually(func() bool { return a.blastCount() > 0 && b.blastCount() > 0 },
		2*time.Second, 10*time.Millisecond, "a code-less extras event broadcasts to every universe")

	// cerebro-internal events must NOT leak to Extras. Send them, then another blast: by
	// the time the trailing blast lands (Eventually below), the internal events have been
	// processed, so otherCount must still be 0.
	eng.Listen(ctx, market.ChangeBalanceEvent{Balance: decimal.NewFromInt(100)})
	eng.Listen(ctx, market.ChangeOrderEvent{ID: "x"})
	eng.Listen(ctx, market.FeedStatusEvent{State: market.FeedConnected})
	eng.Listen(ctx, blastEvent{val: 10})
	is.Eventually(func() bool { return a.blastCount() >= 2 && b.blastCount() >= 2 },
		2*time.Second, 10*time.Millisecond, "trailing blast lands after the internal events")
	is.Zero(a.otherCount()+b.otherCount(), "cerebro-internal events must not leak to Extras")

	cancel()
	eng.Wait()
}
