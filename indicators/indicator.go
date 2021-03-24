package indicators

import (
	"time"

	"github.com/gobenpark/trader/container"
)

type Indicator interface {
	Calculate(container container.Container)
	Get() []Indicate
}

type Indicate struct {
	Data float64
	Date time.Time
}
