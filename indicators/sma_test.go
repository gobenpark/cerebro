package indicators

import (
	"fmt"
	"testing"
	"time"

	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
)

func TestNewIndicate_SMA(t *testing.T) {
	s := NewSma(100)

	container := datacontainer.NewDataContainer()

	for i := float64(1); i < 10000; i++ {
		container.Add(domain.Candle{
			Code:   "1234",
			Low:    i,
			High:   i,
			Open:   i,
			Close:  i,
			Volume: i,
			Date:   time.Now(),
		})
	}

	s.Set(container)

	for _, i := range s.Get() {
		fmt.Printf("%f , %v\n", i.Data, i.Date)
	}

}
