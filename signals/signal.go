package signals

import (
	"context"
	"sync"

	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
)

type ValueType int

const (
	Volume ValueType = iota + 1
	Price
)

type Signal interface {
	Target(ctx context.Context, value indicator.Value)
	Action(ctx context.Context)
}

type Engine struct {
	sg []Signal
	mu sync.RWMutex
}

func (e2 *Engine) Listen(e interface{}) {
	panic("implement me")
}

func NewEngine(sg ...Signal) engine.Engine {
	return &Engine{sg: sg}
}

func (e *Engine) Spawn(ctx context.Context, tick <-chan indicator.Tick, it []item.Item) error {
	chans := map[string]chan indicator.Tick{}
	for _, i := range it {
		chans[i.Code] = make(chan indicator.Tick, 1)
	}

	go func() {
		for i := range tick {
			e.mu.RLock()
			chans[i.Code] <- i
			e.mu.RUnlock()
		}

		//수정 필요함.
		//for i := range e.sg {
		//	e.sg[i].Target(ctx, NewSignalValue(chans[i.Code]))
		//}

		for k, v := range chans {
			close(v)
			delete(chans, k)
		}
	}()
	return nil
}
