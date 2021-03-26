## Live Trader
---
[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-1f425f.svg)](http://golang.org)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/gobenpark/trader.svg)](https://github.com/gobenpark/trader)
[![GitHub release](https://img.shields.io/github/release/gobenpark/trader.js.svg)](https://github.com/gobenpark/trader/releases/)
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







## Version

1. MAJOR version when you make incompatible API changes,
2. MINOR version when you add functionality in a backwards compatible manner, and
3. PATCH version when you make backwards compatible bug fixes.
Additional labels for pre-release and build metadata are available as extensions to the MAJOR.MINOR.PATCH format.

https://semver.org
