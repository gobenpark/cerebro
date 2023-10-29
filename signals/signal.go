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

type Engine struct {
}

func NewEngine() engine.Engine {
	return &Engine{}
}

func (e *Engine) Spawn(ctx context.Context, tick <-chan indicator.Tick, item []item.Item) error {

	return nil
}

type Signal interface {
	Volume() SignalValue
	Price() SignalValue
}

type signal struct {
	tk    <-chan indicator.Tick
	child []chan indicator.Tick
	mu    sync.RWMutex
}

func NewSignal(tick <-chan indicator.Tick) Signal {
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

func (s *signal) Volume() SignalValue {
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
	return SignalValue{downstream}
}

func (s *signal) Price() SignalValue {
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
	return SignalValue{downstream}
}

type SignalValue struct {
	value <-chan float64
}

func (s SignalValue) Mean(d time.Duration) SignalValue {
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
	return SignalValue{value: downstream}
}

// Roi is rate of increase or decrease per duration
// Roi is calculated by (end - start) / start * 100 (%)
// return every tick
func (s SignalValue) ROI(d time.Duration) SignalValue {
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
	return SignalValue{value: downstream}
}
