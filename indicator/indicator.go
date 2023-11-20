package indicator

import (
	"sync"
	"time"

	"github.com/samber/lo" //nolint:depguard
)

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
			default:
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
	downstream := make(chan float64, 1)
	s.mu.Lock()
	s.childs = append(s.childs, childTk)
	s.mu.Unlock()
	go func() {
		defer close(downstream)
		for msg := range childTk {
			downstream <- float64(msg.Volume)
		}
	}()
	return Indicator{downstream}
}

func (s *value) Price() Indicator {
	childTk := make(chan Tick, 1)
	downstream := make(chan float64, 1)
	s.mu.Lock()
	s.childs = append(s.childs, childTk)
	s.mu.Unlock()
	go func() {
		defer close(downstream)
		for msg := range childTk {
			downstream <- float64(msg.Price)
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
	value <-chan float64
}

// Mean is average of tick
func (s Indicator) Mean(d time.Duration) Indicator {
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
	return Indicator{value: downstream}
}

// Roi is rate of increase or decrease per duration
// Roi is calculated by (end - start) / start * 100 (%)
// return every tick
func (s Indicator) ROI(d time.Duration) Indicator {
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
	return Indicator{value: downstream}
}

//
//func (s Indicator) BollingerBand(period int, f func(p float64, t, m, b []Indicate[float64]) bool) Indicator {
//	downstream := make(chan float64, 1)
//	go func() {
//		defer close(downstream)
//		for v := range s.value {
//			t, m, b := BollingerBand(period, s.p)
//			if f(v, t, m, b) {
//				downstream <- v
//			}
//		}
//	}()
//	return Indicator{value: downstream, candles: s.candles}
//}
