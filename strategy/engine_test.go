package strategy_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	marketmock "github.com/gobenpark/cerebro/market/mock"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/strategy"
)

// stubStrategy is an inert strategy; its Next blocks until the context is canceled.
type stubStrategy struct{}

func (stubStrategy) Name() string { return "stub" }
func (stubStrategy) Next(ctx context.Context, _ *item.Item, _ <-chan indicator.Tick, _ *broker.Broker) {
	<-ctx.Done()
}
func (stubStrategy) NotifyOrder(order.Order) {}
func (stubStrategy) NotifyTrade()            {}
func (stubStrategy) NotifyFund()             {}

type noopLogger struct{}

func (noopLogger) Error(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Panic(string, ...any) {}

// TestEngine_ConcurrentSpawnAndListen drives a writer (Spawn) and reader (Listen)
// against the channels map simultaneously. Without the RWMutex this panics with
// "concurrent map read and write" under -race. Empty strategies keep manager()
// trivial (no Subscribe/Next/sleep), so no goroutines leak.
func TestEngine_ConcurrentSpawnAndListen(t *testing.T) {
	defer goleak.VerifyNone(t)

	eng := strategy.NewEngine(noopLogger{}, nil, nil, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	items := []*item.Item{{Code: "AAA"}, {Code: "BBB"}, {Code: "CCC"}}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		eng.Spawn(ctx, items)
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
// market's Subscribe fails, the engine must not leave strategy Next goroutines or
// registered channels behind. goleak fails if a runner leaked.
func TestEngine_SubscribeFailureStartsNoRunners(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mk := marketmock.NewMockMarket(ctrl)
	mk.EXPECT().Subscribe(gomock.Any()).Return(errors.New("boom")).AnyTimes()

	eng := strategy.NewEngine(noopLogger{}, nil, []strategy.Strategy{stubStrategy{}}, mk, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn throttles briefly, then Subscribe fails; the per-item runners must not
	// start, so Wait returns immediately because nothing was launched.
	eng.Spawn(ctx, []*item.Item{{Code: "AAA"}})
	eng.Wait()
}
