package store

import (
	"context"
	"github.com/BumwooPark/trader/store/model"
	"time"
)

type Storer interface {
	// Start Market data stream
	Start(ctx context.Context)

	// Stop All Market data stream
	Stop()

	// Get Market Stream channel
	Data() <-chan model.Chart
}

type store struct {
	input chan model.Chart
}

func NewStore() Storer {
	ch := make(chan model.Chart, 100)
	return &store{input: ch}
}

func (s *store) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				break
			default:
				time.Sleep(1 * time.Second)
				s.input <- model.Chart{
					Low:    1,
					High:   2,
					Open:   3,
					Close:  4,
					Volume: 5,
				}
			}
		}
	}()
}

func (s *store) Stop() {
}

func (s *store) Data() <-chan model.Chart {
	return s.input
}
