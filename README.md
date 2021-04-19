## Live Trader

golang live trading framework
---
[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-1f425f.svg)](http://golang.org)
[![codecov](https://codecov.io/gh/gobenpark/trader/branch/master/graph/badge.svg?token=4UWNV7BMZ3)](https://codecov.io/gh/gobenpark/trader)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/gobenpark/trader.svg)](https://github.com/gobenpark/trader)
[![GitHub release](https://img.shields.io/github/v/release/gobenpark/trader)](https://github.com/gobenpark/trader/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/gobenpark/trader?style=flat-square)](https://goreportcard.com/report/github.com/gobenpark/trader)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/gobenpark/trader)
[![LICENSE](https://img.shields.io/github/license/gobenpark/trader.svg?style=flat-square)](https://github.com/gobenpark/trader/blob/master/LICENSE)

## Introduce
This project was inspired by [backtrader](https://www.backtrader.com)


python backtrader is a great project but it has disadvantage of python GIL 
so i want solve by golang


## Installation

`go get github.com/gobenpark/trader`

## Usage

🙏 Plz wait for beta version 

### 1. first implement interface `store.Store` 

```go
package main

import (
	"context"
	"sort"
	"time"

	"github.com/gobenpark/proto/stock"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
)

type store struct {
	name string
	uid  string
	cli  Client
}

func NewStore(name string) *store {
	cli, err := stock.NewSocketClient(context.Background(), "localhost:50051")
	if err != nil {
		panic(err)
	}

	return &store{name, uuid.NewV4().String(), cli}
}

func (s *store) LoadHistory(ctx context.Context, code string, du time.Duration) ([]container.Candle, error) {
	panic("implement me ")
}

func (s *store) LoadTick(ctx context.Context, code string) (<-chan container.Tick, error) {
	panic("implement me ")
}

func (s *store) Order(code string, ot order.OType, size int64, price float64) error {
	panic("implement me ")
}

func (s *store) Cancel(id string) error {
	panic("implement me ")
}

func (s *store) Uid() string {
	return s.uid
}
```

### 2. implement your own strategy
```go

import (
	"fmt"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/indicators"
	"github.com/gobenpark/trader/order"
)

type Bighands struct {
	Broker broker.Broker
	indi   indicators.Indicator
}

func (s *Bighands) Next(broker *broker.Broker, container container.Container) {
	rsi := indicators.NewRsi(14)
	rsi.Calculate(container)
	fmt.Println(rsi.Get()[0])

	fmt.Println(broker.GetCash())

	obv := indicators.NewObv()

	obv.Calculate(container)
	fmt.Println(obv.Get()[0])
	fmt.Println(container.Code())

	
	sma := indicators.NewSma(20)
	sma.Calculate(container)
	fmt.Println(sma.Get()[0])
	fmt.Println(container.Code())
}

func (s *Bighands) NotifyOrder(o *order.Order) {
	switch o.Status() {
	case order.Submitted:
		fmt.Printf("%s:%s\n", o.Code, "Submitted")
		fmt.Println(o.ExecutedAt)
	case order.Expired:
		fmt.Println("expired")
		fmt.Println(o.ExecutedAt)
	case order.Rejected:
		fmt.Println("rejected")
		fmt.Println(o.ExecutedAt)
	case order.Canceled:
		fmt.Println("canceled")
		fmt.Println(o.ExecutedAt)
	case order.Completed:
		fmt.Printf("%s:%s\n", o.Code, "Completed")
		fmt.Println(o.ExecutedAt)
		fmt.Println(o.Price)
		fmt.Println(o.Code)
		fmt.Println(o.Size)
	case order.Partial:
		fmt.Println("partial")
		fmt.Println(o.ExecutedAt)
	}
}

func (s *Bighands) NotifyTrade() {
	panic("implement me")
}

func (s *Bighands) NotifyCashValue() {
	panic("implement me")
}

func (s *Bighands) NotifyFund() {
	panic("implement me")
}

```

### 3. using cerebro !!

```go
package main

import (
	"time"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/strategy"
)

func main() {
	bk := broker.NewBroker(100000, 0.0005)

	upbit := NewStore("binance")

	smart := &strategy.Bighands{
		Broker: bk,
	}

	cb := cerebro.NewCerebro(
		cerebro.WithBroker(bk),
		cerebro.WithStore(upbit, "KRW-MFT", "KRW-LBC"),
		cerebro.WithStrategy(smart),
		cerebro.WithResample("KRW-MFT", time.Minute*3, true),
		cerebro.WithResample("KRW-LBC", time.Minute*3, true),
		cerebro.WithLive(true),
		cerebro.WithPreload(true),
	)

	err := cb.Start()
	if err != nil {
		panic(err)
	}
}

```


## Concepts

Live Trader have several part of trading components 

1. **Cerebro**
: the **Cerebro**  managements all trading components and make dependency graph

2. **Store** components is user base implements for external real server 
ex) Binance , upbit , etc 
   
3. **Strategy** is user base own strategy

## Feature
1. Indicator
    - bollinger band
    - RSI
    - Simple Moving Average
    - On Balance Bolume
    

## TODO

1. new feature Observer is observing price and volume for find big hands 
2. new feature Signal
3. support news, etc information base trading 
4. multi store one cerebro 
5. feature broker slippage
6. Chart 




## Version

1. MAJOR version when you make incompatible API changes,
2. MINOR version when you add functionality in a backwards compatible manner, and
3. PATCH version when you make backwards compatible bug fixes.
Additional labels for pre-release and build metadata are available as extensions to the MAJOR.MINOR.PATCH format.

https://semver.org



## Contribute
if you have any idea and want to talk this project send me mail (qjadn0914@naver.com) or issue   
