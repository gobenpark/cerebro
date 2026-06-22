package strategy_test

import (
	"context"
	"sync"
	"testing"

	"go.uber.org/goleak"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/strategy"
)

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
