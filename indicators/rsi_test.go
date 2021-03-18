package indicators

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
	"github.com/stretchr/testify/assert"
)

func TestRsi(t *testing.T) {

	f, err := os.Open("ticksample.csv")
	assert.NoError(t, err)

	reader := csv.NewReader(f)
	data, err := reader.ReadAll()
	assert.NoError(t, err)

	container := datacontainer.NewDataContainer()

	stof := func(s string) float64 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}

	rsi := NewRsi(14)
	for _, i := range data[1:] {
		ti, err := time.Parse("2006-01-02T15:04:05.999", i[0])
		assert.NoError(t, err)
		container.Add(domain.Candle{
			Code:   "1234",
			Low:    stof(i[3]),
			High:   stof(i[2]),
			Open:   stof(i[1]),
			Close:  stof(i[4]),
			Volume: stof(i[5]),
			Date:   ti,
		})
	}

	rsi.Calculate(container)
	for _, i := range rsi.Get() {
		fmt.Println(i)
	}
}

func TestSlice(t *testing.T) {
	data := []int{1, 2, 3}
	fmt.Println(data[2:4])
}
