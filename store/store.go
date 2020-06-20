package store

import "github.com/BumwooPark/trader/store/model"

type Storer interface {
}

type store struct {
	input chan model.Chart
}

func NewStore() Storer {
	ch := make(chan model.Chart, 100)
	return &store{input: ch}
}
