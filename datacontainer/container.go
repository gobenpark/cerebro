package datacontainer

import (
	"github.com/gobenpark/trader/domain"
)

type SaveMode int

const (
	InMemory = iota
	External
)

//TODO: inmemory or external storage
type DataContainer struct {
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

func (t *DataContainer) Add(candle domain.Candle) {
	t.CandleData = append([]domain.Candle{candle}, t.CandleData...)
}
