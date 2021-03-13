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
	CandleData map[string][]domain.Candle
}

func (t *DataContainer) Empty() bool {
	return len(t.CandleData) == 0
}

func (t *DataContainer) Size() int {
	return len(t.CandleData)
}

func (t *DataContainer) Clear() {
	t.CandleData = map[string][]domain.Candle{}
}

func (t *DataContainer) Values(code string) []domain.Candle {
	return t.CandleData[code]
}

func (t *DataContainer) Add(candle domain.Candle) {
	if d, ok := t.CandleData[candle.Code]; ok {
		t.CandleData[candle.Code] = append([]domain.Candle{candle}, d...)
	} else {
		t.CandleData[candle.Code] = []domain.Candle{candle}
	}
}
