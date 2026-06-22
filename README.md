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

## How it works

To run Cerebro you provide two things:

1. A **`market.Market`** implementation — your adapter to a real exchange
   (Binance, Upbit, a brokerage API, …). It feeds candles/ticks and order/balance
   events into Cerebro and executes the orders the broker submits.
2. One or more **`strategy.Strategy`** implementations — your trading logic. Each
   strategy receives ticks and places orders through the `*broker.Broker` handed
   to it.

Cerebro wires these together with an internal **broker** (cash/position
accounting) and an **event engine** (per-listener event dispatch), then runs the
whole graph until the context is canceled.

### 1. Implement the `market.Market` interface

```go
package main

import (
	"context"

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
//	Subscribe(event any) error
//	Order(ctx, o order.Order) error
//	AccountPositions() []position.Position
//	AccountBalance() int64
//	Events(ctx) <-chan any
//	Commission() float64
type exchange struct{}

func (e *exchange) Stocks(ctx context.Context) []*item.Item { panic("implement me") }

func (e *exchange) Candles(ctx context.Context, code string, level market.CandleType) (indicator.Candles, error) {
	panic("implement me")
}

// Subscribe is called once per target item; start streaming its ticks here.
func (e *exchange) Subscribe(event any) error { panic("implement me") }

// Order submits an order to the exchange.
func (e *exchange) Order(ctx context.Context, o order.Order) error { panic("implement me") }

func (e *exchange) AccountPositions() []position.Position { panic("implement me") }
func (e *exchange) AccountBalance() int64                 { panic("implement me") }

// Events streams market events to Cerebro: indicator.Tick for price updates and
// market.ChangeOrderEvent / market.ChangeBalanceEvent for fills and settlement.
func (e *exchange) Events(ctx context.Context) <-chan any { panic("implement me") }

// Commission is the percentage fee applied to an order's value.
func (e *exchange) Commission() float64 { panic("implement me") }
```

### 2. Implement your own strategy

`Next` runs in its own goroutine and receives ticks until the context is
canceled. Use the indicators on `indicator.Candles` and place orders through the
broker. `NotifyOrder` is called whenever one of your orders changes state.

```go
package main

import (
	"context"

	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
)

type MyStrategy struct{}

func (s *MyStrategy) Name() string { return "my-strategy" }

func (s *MyStrategy) Next(ctx context.Context, it *item.Item, tick <-chan indicator.Tick, b *broker.Broker) {
	for {
		select {
		case <-ctx.Done():
			return
		case tk, ok := <-tick:
			if !ok {
				return
			}

			// Example: buy 10 units at the current price when cash allows.
			if b.Available() >= tk.Price*10 {
				o := order.NewOrder(it, order.Buy, order.Limit, 10, tk.Price)
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
	"os"
	"os/signal"
	"time"

	"github.com/gobenpark/cerebro"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/log"
)

func main() {
	cb := cerebro.NewCerebro(
		cerebro.WithMarket(&exchange{}),
		cerebro.WithStrategy(&MyStrategy{}),
		cerebro.WithTargetItem(
			&item.Item{Code: "KRW-BTC"},
			&item.Item{Code: "KRW-ETH"},
		),
		cerebro.WithStrategyTimeout(5*time.Second),
		cerebro.WithLogLevel(log.InfoLevel),
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
| `WithStrategy(...strategy.Strategy)` | One or more strategies (required). |
| `WithTargetItem(...*item.Item)` | Items to trade (required). |
| `WithStrategyTimeout(time.Duration)` | Per-strategy `Next` timeout budget. |
| `WithLogLevel(log.Level)` | Log verbosity. |

## Concepts

Cerebro is composed of a few cooperating parts:

1. **Cerebro** — the orchestrator. It builds the dependency graph, starts every
   component, and tears them down in order on shutdown.
2. **Market** — a user-implemented adapter to an external exchange (candles,
   ticks, order execution, account state, and an event stream).
3. **Strategy** — your trading logic. Each strategy runs as its own goroutine and
   receives a private tick channel, so one slow strategy never starves another.
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
- **Resampling** — build candles from raw ticks (`indicator.Resample`,
  `indicator.Resampler`).
- **Concurrency** — event-driven core with per-listener dispatch and graceful,
  ordered shutdown (producers stop first, the dispatcher last).

## Roadmap

1. Observer for detecting unusual price/volume activity ("big hands")
2. Signal abstraction
3. News / external-information based trading
4. Broker slippage modeling
5. Charting

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
