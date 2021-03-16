package indicators

import "github.com/gobenpark/trader/datacontainer"

type Indicator interface {
	Set(container datacontainer.DataContainer)
	Get()
	PeriodSatisfaction() bool
}

type Sma struct {
}
