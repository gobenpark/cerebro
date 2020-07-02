package strategy

import (
	"fmt"
	"github.com/BumwooPark/trader/store/model"
)

type Smart struct {
	Chart chan model.Chart
}

func NewSmartStrategy() *Smart {
	ch := make(chan model.Chart, 100)
	return &Smart{ch}
}

func (s *Smart) ChartChannel() chan<- model.Chart {
	return s.Chart
}

func (s *Smart) Logic() {
	go func() {
		for i := range s.Chart {
			fmt.Printf("%#v\n", i)
		}
	}()
}
