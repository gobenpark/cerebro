package indicators

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
)

func TestRsi(t *testing.T) {
	container := datacontainer.NewDataContainer()
	ran := func() float64 {
		n, err := rand.Int(rand.Reader, big.NewInt(100))
		if err != nil {
			panic(err)
		}
		return float64(n.Int64())
	}
	rsi := NewRsi(0)

	for i := 0; i < 100; i++ {
		container.Add(domain.Candle{
			Code:   "1234",
			Low:    ran(),
			High:   ran(),
			Open:   ran(),
			Close:  ran(),
			Volume: ran(),
			Date:   time.Now(),
		})
	}

	rsi.Calculate(container)
	fmt.Println(rsi.Get())
}
