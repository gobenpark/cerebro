package signals

import (
	"context"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/engine"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/samber/lo"
)

type ValueType int

const (
	Volume ValueType = iota + 1
	Price
)

type Signal interface {
	Target(ctx context.Context, value SignalValue)
	Action(ctx context.Context)
}

type Engine struct {
	sg []Signal
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
			chans[i.Code] <- i
		}

		//수정 필요함.
		for i := range e.sg {
			e.sg[i].Target(ctx, NewSignalValue(chans[i.Code]))
		}

		for k, v := range chans {
			close(v)
			delete(chans, k)
		}
	}()
	return nil
}

type SignalValue interface {
	Volume() SignalAction
	Price() SignalAction
}

type signal struct {
	tk    <-chan indicator.Tick
	child []chan indicator.Tick
	mu    sync.RWMutex
}

func NewSignalValue(tick <-chan indicator.Tick) SignalValue {
	sg := &signal{tk: tick}
	go func() {
		for t := range tick {
			for i := range sg.child {
				sg.mu.RLock()
				sg.child[i] <- t
				sg.mu.RUnlock()
			}
		}
		for i := range sg.child {
			close(sg.child[i])
		}
	}()

	return sg
}

func (s *signal) Volume() SignalAction {
	childTk := make(chan indicator.Tick, 1)
	downstream := make(chan float64, 1)
	s.mu.Lock()
	s.child = append(s.child, childTk)
	s.mu.Unlock()
	go func() {
		defer close(downstream)
		for msg := range childTk {
			downstream <- float64(msg.Volume)
		}
	}()
	return SignalAction{downstream}
}

func (s *signal) Price() SignalAction {
	childTk := make(chan indicator.Tick, 1)
	downstream := make(chan float64, 1)
	s.mu.Lock()
	s.child = append(s.child, childTk)
	s.mu.Unlock()
	go func() {
		defer close(downstream)
		for msg := range childTk {
			downstream <- float64(msg.Price)
		}
	}()
	return SignalAction{downstream}
}

type SignalAction struct {
	value <-chan float64
}

func (s SignalAction) Mean(d time.Duration) SignalAction {
	downstream := make(chan float64, 1)
	tk := []float64{}
	ticker := time.NewTicker(d)

	go func() {
		defer close(downstream)
	Done:
		for {
			select {
			case <-ticker.C:
				sum := lo.Sum(tk)
				if sum == 0 || len(tk) == 0 {
					downstream <- 0
					continue
				}
				value := sum / float64(len(tk))
				downstream <- value
				tk = []float64{}
			case tick, ok := <-s.value:
				if !ok {
					break Done
				}
				tk = append(tk, tick)
			}
		}
	}()
	return SignalAction{value: downstream}
}

// Roi is rate of increase or decrease per duration
// Roi is calculated by (end - start) / start * 100 (%)
// return every tick
func (s SignalAction) ROI(d time.Duration) SignalAction {
	downstream := make(chan float64, 1)
	start := float64(0)
	end := float64(0)
	ticker := time.NewTicker(d)

	go func() {
		defer close(downstream)
	Done:
		for {
			select {
			case <-ticker.C:
				start = end
			case v, ok := <-s.value:
				if !ok {
					break Done
				}
				end = v
				if v == 0 {
					continue
				}

				if start == 0 {
					start = v
					continue
				}

				downstream <- (end - start) / start * 100
			}
		}
	}()
	return SignalAction{value: downstream}
}

func (s SignalAction) Greater(then SignalAction) bool {
	return false
}
