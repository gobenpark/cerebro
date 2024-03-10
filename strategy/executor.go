package strategy

import "github.com/gobenpark/cerebro/indicator"

type Executor interface {
	Use(i ...indicator.Indicator)
}

type executor struct {
	engine []indicator.Indicator
}

func (e *executor) Use(i ...indicator.Indicator) {
	e.engine = append(e.engine, i...)
}
