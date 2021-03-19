package datacontainer

import (
	"sync"

	"github.com/gobenpark/trader/domain"
)

type SaveMode int

const (
	InMemory = iota
	External
)

//TODO: inmemory or external storage
type DataContainer struct {
	mu         sync.Mutex
	CandleData []domain.Candle
}

func NewDataContainer() *DataContainer {
	return &DataContainer{CandleData: []domain.Candle{}}
}

func (t *DataContainer) Empty() bool {
	return len(t.CandleData) == 0
}

func (t *DataContainer) Size() int {
	return len(t.CandleData)
}

func (t *DataContainer) Clear() {
	t.CandleData = []domain.Candle{}
}

func (t *DataContainer) Values() []domain.Candle {
	return t.CandleData
}

//Add forword append container candle data
// current candle [0] index
func (t *DataContainer) Add(candle domain.Candle) {
	if len(t.CandleData) != 0 {
		for _, i := range t.CandleData {
			if i.Date.Equal(candle.Date) {
				return
			}
		}
	}
	t.CandleData = append([]domain.Candle{candle}, t.CandleData...)
}
