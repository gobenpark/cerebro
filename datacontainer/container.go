package datacontainer

import (
	"errors"

	"github.com/gobenpark/trader/domain"
)

type SaveMode int

const (
	InMemory = iota
	External
)

type DataContainer struct {
	candledata []domain.Candle
	SaveMode
}

func (t *DataContainer) Empty() bool {
	return len(t.candledata) == 0
}

func (t *DataContainer) Size() int {
	return len(t.candledata)
}

func (t *DataContainer) Clear() {
	t.candledata = []domain.Candle{}
}

func (t *DataContainer) Values() []interface{} {
	d := make([]interface{}, len(t.candledata))
	for k, v := range t.candledata {
		d[k] = v
	}
	return d
}

func (t *DataContainer) Add(data interface{}) error {
	if cd, ok := data.(domain.Candle); ok {
		t.candledata = append([]domain.Candle{cd}, t.candledata...)
		return nil
	}
	return errors.New("datacontainer is not domain.Tick")
}
