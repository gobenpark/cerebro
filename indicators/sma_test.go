package indicators

import (
	"testing"
	"time"

	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewIndicate_SMA(t *testing.T) {
	s := NewSma(3)

	container := datacontainer.NewDataContainer("code")

	for i := float64(1); i <= 10; i++ {
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

	s.Calculate(container)
	assert.Len(t, s.Get(), 8)

	container.Add(domain.Candle{
		Code:   "1234",
		Low:    11,
		High:   11,
		Open:   11,
		Close:  11,
		Volume: 11,
		Date:   time.Now(),
	})

	s.Calculate(container)
	assert.Len(t, s.Get(), 9)
}
