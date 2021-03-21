package datacontainer

import (
	"sync"
	"time"

	"github.com/gobenpark/trader/domain"
)

type SaveMode int

const (
	InMemory = iota
	External
)

type ContainerInfo struct {
	Code             string
	CompressionLevel time.Duration
}

//TODO: inmemory or external storage
type DataContainer struct {
	mu         sync.Mutex
	CandleData []domain.Candle
	ContainerInfo
}

func NewDataContainer(info ContainerInfo) *DataContainer {
	return &DataContainer{
		CandleData:    []domain.Candle{},
		ContainerInfo: info,
	}
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
	t.mu.Lock()
	d := make([]domain.Candle, len(t.CandleData))
	copy(d, t.CandleData)
	t.mu.Unlock()
	return d
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
	t.mu.Lock()
	t.CandleData = append([]domain.Candle{candle}, t.CandleData...)
	t.mu.Unlock()
}

func (t *DataContainer) Code() string {
	return t.ContainerInfo.Code
}

func (t *DataContainer) Level() time.Duration {
	return t.ContainerInfo.CompressionLevel
}
