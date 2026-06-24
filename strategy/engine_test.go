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
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
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
func (stubStrategy) NotifyTrade()            {}
func (stubStrategy) NotifyFund()             {}

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
func (s *recordingStrategy) NotifyTrade()            {}
func (s *recordingStrategy) NotifyFund()             {}
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
func (s *tickRecorder) NotifyTrade()            {}
func (s *tickRecorder) NotifyFund()             {}
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
