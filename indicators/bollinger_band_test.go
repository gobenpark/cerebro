package indicators

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gobenpark/trader/container"
	"github.com/stretchr/testify/assert"
)

func TestBollingerBand_Calculate(t *testing.T) {

	f, err := os.Open("ticksample.csv")
	assert.NoError(t, err)

	reader := csv.NewReader(f)
	data, err := reader.ReadAll()
	assert.NoError(t, err)

	c := container.NewDataContainer(container.Info{
		Code:             "code",
		CompressionLevel: 0,
	})

	stof := func(s string) float64 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}

	b := NewBollingerBand(20)

	for _, i := range data[1:] {
		ti, err := time.Parse("2006-01-02T15:04:05Z", i[6])
		assert.NoError(t, err)
		c.Add(container.Candle{
			Code:   i[0],
			Low:    stof(i[3]),
			High:   stof(i[2]),
			Open:   stof(i[1]),
			Close:  stof(i[4]),
			Volume: stof(i[5]),
			Date:   ti,
		})
	}
	b.Calculate(c)
}

func TestRoot(t *testing.T) {
	fmt.Println(math.Sqrt(4))
	fmt.Println(math.Pow(-10, 2))
}
