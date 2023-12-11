package indicator

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/samber/lo" //nolint:depguard
)

type Packet struct {
	Tick  Tick
	Value float64
}

// InternalIndicator only for internal use
type InternalIndicator interface {
	Value
	Start(tick <-chan Tick)
}

type Value interface {
	Volume() Indicator
	Price() Indicator
	Filter(f func(Tick) bool) Value
	Copy() Value
}

type value struct {
	tk     <-chan Tick
	childs []chan Tick
	mu     sync.RWMutex
}

func NewValue() InternalIndicator {
	return &value{}
}

func (v *value) Start(tick <-chan Tick) {
	go func() {
		for {
			select {
			case t, ok := <-tick:
				if !ok {
					return
				}
				for i := range v.childs {
					v.childs[i] <- t
				}
			}
		}

		for i := range v.childs {
			close(v.childs[i])
		}
	}()
}

func (s *value) Copy() Value {
	childTk := make(chan Tick, 1)
	downstream := make(chan Tick, 1)
	s.mu.Lock()
	s.childs = append(s.childs, childTk)
	s.mu.Unlock()

	v := value{
		tk:     downstream,
		childs: []chan Tick{},
	}

	go func() {
		for tk := range childTk {
			for i := range v.childs {
				v.childs[i] <- tk
			}
		}
	}()

	return &v
}

func (s *value) Volume() Indicator {
	childTk := make(chan Tick, 1)
	downstream := make(chan Packet, 1)
	s.mu.Lock()
	s.childs = append(s.childs, childTk)
	s.mu.Unlock()
	go func() {
		defer close(downstream)
		for msg := range childTk {
			downstream <- Packet{
				Value: float64(msg.Volume),
				Tick:  msg,
			}
		}
	}()
	return Indicator{downstream}
}

func (s *value) Price() Indicator {
	childTk := make(chan Tick, 1)
	downstream := make(chan Packet, 1)
	s.mu.Lock()
	s.childs = append(s.childs, childTk)
	s.mu.Unlock()
	go func() {
		defer close(downstream)
		for msg := range childTk {
			downstream <- Packet{
				Value: float64(msg.Price),
				Tick:  msg,
			}
		}
	}()
	return Indicator{downstream}
}

func (s *value) Filter(f func(Tick) bool) Value {
	childTk := make(chan Tick, 1)
	downstream := make(chan Tick, 1)
	s.mu.Lock()
	s.childs = append(s.childs, childTk)
	s.mu.Unlock()

	v := value{
		tk:     downstream,
		childs: []chan Tick{},
	}

	go func() {
		for tk := range childTk {
			if f(tk) {
				for i := range v.childs {
					v.childs[i] <- tk
				}
			}
		}
	}()

	return &v
}

type Indicator struct {
	value <-chan Packet
}

// Mean is average of tick
func (s Indicator) Mean(d time.Duration) Indicator {
	downstream := make(chan Packet, 1)
	tk := []float64{}
	ticker := time.NewTicker(d)
	rawdata := Tick{}

	go func() {
		defer close(downstream)
	Done:
		for {
			select {
			case <-ticker.C:
				sum := lo.Sum(tk)
				if sum == 0 || len(tk) == 0 {
					downstream <- Packet{}
					continue
				}
				value := sum / float64(len(tk))
				downstream <- Packet{
					Value: value,
					Tick:  rawdata,
				}
				tk = []float64{}
			case tick, ok := <-s.value:
				if !ok {
					break Done
				}
				rawdata = tick.Tick
				tk = append(tk, tick.Value)
			}
		}
	}()
	return Indicator{value: downstream}
}

func (s Indicator) Filter(f func(value Packet) bool) Indicator {
	downstream := make(chan Packet, 1)
	go func() {
		defer close(downstream)
		for msg := range s.value {
			if f(msg) {
				downstream <- msg
			}
		}
	}()
	return Indicator{value: downstream}
}

// Roi is rate of increase or decrease per duration
// Roi is calculated by (end - start) / start * 100 (%)
// return every tick
func (s Indicator) ROI(d time.Duration) Indicator {
	downstream := make(chan Packet, 1)
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
				end = v.Value
				if v.Value == 0 {
					continue
				}

				if start == 0 {
					start = v.Value
					continue
				}

				downstream <- Packet{
					Value: (end - start) / start * 100,
					Tick:  v.Tick,
				}
			}
		}
	}()
	return Indicator{value: downstream}
}

func (s Indicator) LargeThen(i Indicator) {
	downstream := make(chan Packet, 1)
	var mu sync.RWMutex

	go func() {
		defer close(downstream)
		var value float64 = 0

		go func() {
			for upstream := range s.value {
				mu.RLock()
				if upstream.Value > value {
					downstream <- upstream
				}
				mu.RUnlock()
			}
		}()

		for upstream := range i.value {
			mu.Lock()
			value = upstream.Value
			mu.Unlock()
		}
	}()
}

func (s Indicator) Transaction(f func(v Packet)) {
	go func() {
		for v := range s.value {
			f(v)
		}
	}()
}

func CombineWithF(f func(v ...float64) float64, indicators ...Indicator) Indicator {
	downstream := make(chan Packet, 1)
	go func() {
		var wg sync.WaitGroup
		size := uint32(len(indicators))
		var counter uint32
		mutex := sync.Mutex{}
		s := make([]float64, size)

		handler := func(i Indicator, idx int) {
			for v := range i.value {

				if s[idx] == 0 {
					atomic.AddUint32(&counter, 1)
				}

				mutex.Lock()
				s[idx] = v.Value
				if atomic.LoadUint32(&counter) == size {
					downstream <- Packet{Value: f(s...), Tick: v.Tick}
					atomic.StoreUint32(&counter, 0)
					s = make([]float64, size)
				}
				mutex.Unlock()
			}
			wg.Done()
		}

		for i := range indicators {
			wg.Add(1)
			go handler(indicators[i], i)
		}
		wg.Wait()
		defer close(downstream)
	}()
	return Indicator{value: downstream}
}
