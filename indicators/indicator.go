package indicators

import (
	"time"

	"github.com/gobenpark/trader/domain"
)

type Indicator interface {
	Calculate(container domain.Container)
	Get() []Indicate
	PeriodSatisfaction() bool
}

type Indicate struct {
	Data float64
	Date time.Time
}

type Sma struct {
}
