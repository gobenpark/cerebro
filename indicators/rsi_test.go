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

	container := datacontainer.NewDataContainer("code")

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

func TestSlice2(t *testing.T) {
	data := [][2]float64{
		{331, 331},
		{331, 333},
		{333, 333},
		{334, 334},
		{334, 333},
		{332, 332},
		{333, 332},
		{332, 332},
		{332, 333},
		{332, 332},
		{333, 332},
		{332, 332},
		{332, 333},
		{333, 338},
	}

	date := []string{
		"2021-03-18 08:57:00",
		"2021-03-18 09:00:00",
		"2021-03-18 09:03:00",
		"2021-03-18 09:06:00",
		"2021-03-18 09:09:00",
		"2021-03-18 09:12:00",
		"2021-03-18 09:15:00",
		"2021-03-18 09:18:00",
		"2021-03-18 09:21:00",
		"2021-03-18 09:24:00",
		"2021-03-18 09:27:00",
		"2021-03-18 09:30:00",
		"2021-03-18 09:33:00",
		"2021-03-18 09:36:00",
	}

	container := datacontainer.NewDataContainer("code")
	rsi := NewRsi(14)

	for k, v := range data {

		ti, err := time.Parse("2006-01-02 15:04:05", date[k])
		assert.NoError(t, err)
		container.Add(domain.Candle{
			Code:  "1234",
			Open:  v[0],
			Close: v[1],
			Date:  ti,
		})
	}

	rsi.Calculate(container)
	fmt.Println(rsi.Get())
}
