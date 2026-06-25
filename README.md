## Cerebro

> ⚠️ This project is still in progress and is not yet a stable release. The public API may change.

A Go live-trading framework
---
[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-1f425f.svg)](http://golang.org)
[![codecov](https://codecov.io/gh/gobenpark/cerebro/branch/master/graph/badge.svg?token=4UWNV7BMZ3)](https://codecov.io/gh/gobenpark/cerebro)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/gobenpark/cerebro.svg)](https://github.com/gobenpark/cerebro)
[![GitHub release](https://img.shields.io/github/v/release/gobenpark/cerebro)](https://github.com/gobenpark/cerebro/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/gobenpark/cerebro?style=flat-square)](https://goreportcard.com/report/github.com/gobenpark/cerebro)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/gobenpark/cerebro)
[![LICENSE](https://img.shields.io/github/license/gobenpark/cerebro.svg?style=flat-square)](https://github.com/gobenpark/cerebro/blob/master/LICENSE)

## Introduction

This project was inspired by [backtrader](https://www.backtrader.com).

`backtrader` is a great Python project, but it is constrained by Python's GIL.
Cerebro aims to solve that with Go's concurrency model: an event-driven core
where the market feed, strategies, and broker run as independent, context-aware
goroutines that shut down gracefully in order.

## Installation

```bash
go get github.com/gobenpark/cerebro
```

Requires Go 1.26+.

## Quickstart

The fastest way to see Cerebro run is the bundled **replay market**, which streams
historical candles and simulates fills — no real exchange needed:

```bash
go run ./examples/backtest
```

```
running backtest over 13 bars ...
  notify [dip] AAA status=2
  notify [dip] AAA status=5

final balance: 999029.85
position AAA size=10 avg=97.00
```

That example ([`examples/backtest/main.go`](examples/backtest/main.go)) wires the
replay market, a small dip-buying strategy, and a risk gate through Cerebro and
prints the result. It is the place to start reading; the sections below show how
to plug in a **real** exchange for live trading.

## How it works

To run Cerebro you provide two things:

1. A **`market.Market`** implementation — your adapter to an exchange (Binance,
   Upbit, a brokerage API, …) or the bundled `market/replay` for backtests. It
   feeds candles/ticks and order/balance events into Cerebro and executes the
   orders the broker submits.
2. One or more **`strategy.Strategy`** implementations — your trading logic. Each
   strategy receives ticks and places orders through the `broker.Submitter` handed
   to it (scoped to the strategy, so its orders are attributed to it).

Cerebro wires these together with an internal **broker** (cash/position
accounting) and an **event engine** (per-listener event dispatch), then runs the
whole graph until the context is canceled.

### 1. Implement the `market.Market` interface

```go
package main

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
)

// market.Market interface:
//
//	Stocks(ctx) []*item.Item
//	Candles(ctx, code, level) (indicator.Candles, error)
//	Subscribe(ctx, handler market.TickEventHandler) error
//	Order(ctx, o order.Order) error
//	AccountPositions(ctx) []position.Position
//	AccountBalance(ctx) decimal.Decimal
//	Events(ctx) <-chan any
//	Commission() market.Rate
type exchange struct{}

func (e *exchange) Stocks(ctx context.Context) []*item.Item { panic("implement me") }

func (e *exchange) Candles(ctx context.Context, code string, level market.CandleType) (indicator.Candles, error) {
	panic("implement me")
}

// Subscribe is called once per watchlist item; start streaming its ticks here.
// The handler reports which items to subscribe.
func (e *exchange) Subscribe(ctx context.Context, handler market.TickEventHandler) error {
	panic("implement me")
}

// Order submits an order to the exchange.
func (e *exchange) Order(ctx context.Context, o order.Order) error { panic("implement me") }

func (e *exchange) AccountPositions(ctx context.Context) []position.Position { panic("implement me") }
func (e *exchange) AccountBalance(ctx context.Context) decimal.Decimal       { panic("implement me") }

// Events streams market events to Cerebro: indicator.Tick for price updates,
// indicator.OrderBook for order-book (호가) snapshots, and market.ChangeOrderEvent /
// market.ChangeBalanceEvent for fills and settlement.
// Liveness contract: a live adapter survives a transient disconnect by reconnecting
// internally and keeping this channel open (a drop must not close it), and SHOULD
// emit a market.FeedStatusEvent on disconnect/reconnect — it surfaces feed health and
// resets Cerebro's staleness watchdog (see WithFeedTimeout) during quiet periods.
func (e *exchange) Events(ctx context.Context) <-chan any { panic("implement me") }

// Commission is the fee rate applied to an order's value, as a market.Rate whose
// unit is explicit at the build site — market.Percent(0.15) or market.Fraction(0.0015),
// both 0.15%. It must be a cheap, non-blocking accessor (no I/O) — it is read on
// every fill.
func (e *exchange) Commission() market.Rate { panic("implement me") }
```

### 2. Implement your own strategy

`Run` runs in its own goroutine and decides over a **`strategy.Universe`** — the
set of instruments it trades together plus their merged tick stream — until the
context is canceled. A single-instrument strategy reads `u.Items()[0]` and ranges
over `u.Ticks()`; a pairs/portfolio strategy ranges over `u.Items()` and
demultiplexes `u.Ticks()` by `indicator.Tick.Code`. When the market adapter
publishes order books, `u.OrderBooks()` carries `indicator.OrderBook` snapshots
(bids/asks with `BestBid` / `BestAsk` / `Spread` / `Mid` / `Imbalance` helpers).
Place orders through the broker; `NotifyOrder` is called whenever one of your orders
changes state.

```go
package main

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/strategy"
)

type MyStrategy struct{ code string }

// Name must be unique per running instance. When spawned per item by a WithScreener
// factory, derive it from the item's Code so each instance differs.
func (s *MyStrategy) Name() string { return "my-strategy:" + s.code }

func (s *MyStrategy) Run(ctx context.Context, u strategy.Universe, b broker.Submitter) {
	it := u.Items()[0] // single-instrument strategy
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-u.Ticks():
			if !ok {
				return
			}

			// Example: buy 10 units at the current price when cash allows.
			qty := decimal.NewFromInt(10)
			if b.Available().GreaterThanOrEqual(tk.Price.Mul(qty)) {
				o := order.NewOrder(it, order.Buy, order.Limit, qty, tk.Price)
				if err := b.Order(ctx, o, true /* safe: one open order per code */); err != nil {
					// e.g. broker.ErrNotEnoughMoney
				}
			}
		}
	}
}

func (s *MyStrategy) NotifyOrder(o order.Order) {
	switch o.Status() {
	case order.Submitted:
		// order accepted by the broker and sent to the exchange
	case order.Completed:
		// fully filled
	case order.Canceled, order.Expired, order.Rejected, order.Margin:
		// terminal, non-filled outcomes
	}
}

func (s *MyStrategy) NotifyTrade() {}
func (s *MyStrategy) NotifyFund()  {}
```

### 3. Wire it up with Cerebro

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/gobenpark/cerebro"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/strategy"
)

func main() {
	cb := cerebro.NewCerebro(
		cerebro.WithMarket(&exchange{}),
		// Run a fresh MyStrategy per screened item. StaticScreener wraps a fixed list;
		// swap in a streaming Screener (e.g. a "top by turnover" feed with your filter)
		// for a dynamic, real-time watchlist that spawns and retires per-item strategies
		// as it changes. For one strategy over a fixed, explicit universe instead, use
		// cerebro.WithStrategy(&PairsStrategy{}, "KRW-BTC", "KRW-ETH").
		cerebro.WithScreener(
			cerebro.StaticScreener(
				&item.Item{Code: "KRW-BTC"},
				&item.Item{Code: "KRW-ETH"},
			),
			func(it *item.Item) strategy.Strategy {
				return &MyStrategy{code: it.Code}
			},
		),
		cerebro.WithStrategyTimeout(5*time.Second),
		cerebro.WithLogLevel(slog.LevelInfo),
		// Or route logs into your own pipeline: cerebro.WithLogger(myLogger).
	)

	// Start returns immediately after spawning the producers. Cancel the context
	// (or call cb.Shutdown()) to trigger a graceful, ordered shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := cb.Start(ctx); err != nil {
		panic(err)
	}

	<-ctx.Done() // wait for Ctrl-C
	cb.Shutdown() // blocks until every component has drained
}
```

#### Options

| Option | Description |
| --- | --- |
| `WithMarket(market.Market)` | Exchange adapter (required). |
| `WithStrategy(strategy.Strategy, codes...)` | Register one strategy over a fixed, explicit universe (at least one code; give several for a pairs/portfolio strategy). |
| `WithScreener(cerebro.Screener, factory, ...ScreenOption)` | Register a dynamic screening group: the screener streams watchlist snapshots and a per-item strategy is spawned from `factory` for each, retired (per the eviction policy) when it drops out. Call again for an independent group. `StaticScreener` wraps a fixed list. |
| `WithEviction(cerebro.EvictionPolicy)` | A `WithScreener` group option: what to do with a dropped item's strategy — `KeepUntilFlat` (default), `Flatten`, `DropImmediately`, or your own. |
| `WithStrategyTimeout(time.Duration)` | Per-strategy `Run` timeout budget. |
| `WithRisk(...risk.Rule)` | Pre-trade risk gate (position/order/rate limits). |
| `WithRiskPolicy(name, risk.Policy)` | Per-strategy reactive exit (stop-loss / trailing-stop / take-profit). |
| `WithStorage(broker.Storage)` | Persist/restore the per-strategy ledger (realized PnL, fees, open lots) across restarts. |
| `WithFeedTimeout(time.Duration)` | Arm a live-feed staleness watchdog: trip the feed-loss handler if no tick or `market.FeedStatusEvent` heartbeat arrives in time (off by default; for live feeds, not backtests). |
| `WithFeedLossHandler(func(reason string))` | Handle a lost feed (gone stale, or its channel closed mid-run). Replaces the default fail-safe `Shutdown`. |
| `WithLogLevel(slog.Level)` | Level of the default stderr `slog` logger. |
| `WithLogger(*slog.Logger)` | Route logs through your own `slog.Logger` (use `slog.DiscardHandler` to silence). |

## Concepts

Cerebro is composed of a few cooperating parts:

1. **Cerebro** — the orchestrator. It builds the dependency graph, starts every
   component, and tears them down in order on shutdown.
2. **Market** — a user-implemented adapter to an external exchange (candles,
   ticks, order execution, account state, and an event stream).
3. **Strategy** — your trading logic. Each strategy runs as its own goroutine over
   a **Universe** (one or more instruments and their merged tick stream), so one
   slow strategy never starves another. Register a strategy per item for a
   watchlist, or over several codes for a pairs/portfolio strategy.
4. **Broker** — tracks cash, positions, and open orders. Accounting is
   exchange-authoritative: settled balance comes from the exchange, while open buy
   orders reserve cash so the broker never over-commits before settlement.
5. **Event engine** — fans market and order events out to each listener through a
   dedicated per-listener queue, with context-aware broadcast and drain on
   shutdown.

## Features

- **Indicators** (methods on `indicator.Candles`)
  - Bollinger Band
  - MACD
  - Stochastic (Fast / Slow)
  - Envelope
  - Volume Ratio
  - RMA (Wilder moving average)
- **Decimal money** — prices, sizes, balances, and order values use
  `shopspring/decimal`, so financial math carries no float64 rounding error.
  Commission is a typed `market.Rate` built with `market.Percent` or
  `market.Fraction`, so a fee's unit is explicit at the call site.
- **Replay market** — `market/replay` streams historical candles and simulates
  fills locally, so strategies run end-to-end with no real exchange (see the
  Quickstart). `Done()` signals when a backtest run finishes.
- **Risk gate** — compose pre-trade rules via `cerebro.WithRisk` (`MaxPositionPct`,
  `MaxOrderValue`, `MaxOpenPositions`, `OrderRateLimit`, `MaxLoss`, or custom `risk.Func`).
- **Reactive exit policies** — attach a per-strategy stop-loss / trailing-stop /
  take-profit with `cerebro.WithRiskPolicy`. A monitor tracks each strategy's
  attributed position and submits a market exit on its behalf when a trigger fires.
- **Multi-asset strategies** — a strategy decides over a **Universe** of
  instruments. Register one over several codes for **pairs/portfolio** trading
  (`cerebro.WithStrategy(s, "AAA", "BBB")`) — one `Run` sees every leg's ticks and
  trades them together. See the runnable [`examples/pairs`](examples/pairs/main.go).
- **Dynamic screening** — `cerebro.WithScreener(screener, factory, ...)` registers a
  group: the screener streams watchlist snapshots and a per-item strategy is spawned
  from `factory` for each screened item, then retired — per a `WithEviction` policy
  (`KeepUntilFlat` by default, or `Flatten`) — when it drops out. Register several
  groups for several screener→strategy pipelines; `StaticScreener` wraps a fixed list.
  It is the seam connecting "what to trade" (screening) to "when to trade" (strategy).
- **Order book (호가)** — when the market adapter publishes `indicator.OrderBook`
  snapshots, each strategy receives them for its universe on `u.OrderBooks()`
  (best-first bids/asks with `BestBid` / `BestAsk` / `Spread` / `Mid` / `Imbalance`
  helpers), delivered best-effort alongside ticks. Adapters emit them on the existing
  event stream — no interface change.
- **Strategy attribution** — each strategy submits through a broker handle scoped
  to its `Name()`, so orders and their fills are attributed back to it.
- **Realized PnL & reporting** — the broker books realized PnL and fees per
  strategy from attributed fills (average-cost basis); `Cerebro.Report()` returns
  a per-strategy snapshot of realized PnL, fees, and open positions.
- **Persistence & crash recovery** — attach a `broker.Storage` with
  `cerebro.WithStorage` and the broker restores its per-strategy ledger (realized
  PnL, fees, and open lots) on start and writes it back after each booked fill, so
  attributed trading state survives a restart. `store.NewFileStorage` (atomic JSON
  file) and `store.NewMemoryStorage` ship in the box. Cash balance and account
  positions are exchange-authoritative and re-fetched on start, so they are not
  persisted; in-flight orders are not yet restored.
- **Feed resilience** — arm a market-data staleness watchdog with
  `cerebro.WithFeedTimeout`: if ticks (or a `market.FeedStatusEvent` heartbeat) stop
  flowing, or the feed's channel closes while the run is still live, Cerebro fails
  safe by shutting down — or runs your `cerebro.WithFeedLossHandler`. Live adapters
  reconnect internally and keep their `Events` channel open across transient drops
  (the `market.Market` liveness contract); backtests leave the watchdog off.
- **Resampling** — build OHLCV candles from raw ticks: `indicator.Resample` for a
  batch, or a stateful `indicator.Resampler` for a strategy's tick loop — its
  `Add` reports each bar as it closes, with an optional `WithWindow` memory cap.
- **Concurrency** — event-driven core with per-listener dispatch and graceful,
  ordered shutdown (producers stop first, the dispatcher last).

## Roadmap

Toward production live-trading safety:

1. Runtime kill switch / control surface
2. Broker slippage modeling
3. Open-order persistence and reconciliation on restart

## Versioning

This project follows [Semantic Versioning](https://semver.org):

1. **MAJOR** for incompatible API changes,
2. **MINOR** for backwards-compatible functionality,
3. **PATCH** for backwards-compatible bug fixes.

Additional labels for pre-release and build metadata are available as extensions
to the `MAJOR.MINOR.PATCH` format.

## Contribute

Have an idea or want to discuss the project? Open an issue or send a mail to
qjadn0914@naver.com.
