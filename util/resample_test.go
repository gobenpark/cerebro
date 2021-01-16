package util

import (
	"encoding/csv"
	"fmt"
	"github.com/gobenpark/trader/store/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strconv"
	"testing"
	"time"
)

func MakeBar(bars *[]model.Chart, tick model.Tick) {
	count := len(*bars)
	newChart := model.Chart{
		Low:    tick.Price,
		High:   tick.Price,
		Open:   tick.Price,
		Close:  tick.Price,
		Volume: tick.Volume,
		Date: func() time.Time {
			if tick.Date.Truncate(time.Minute).Add(2*time.Minute).Hour() > tick.Date.Hour() {
				return tick.Date.Truncate(time.Minute)
			}
			return tick.Date.Add(3 * time.Minute).Truncate(time.Minute)
		}(),
	}

	if count == 0 {
		*bars = append(*bars, newChart)
		return
	}

	bar := (*bars)[count-1]
	if bar.Date.Truncate(time.Second).After(tick.Date) && bar.Date.Truncate(time.Second).Add(-3*time.Minute).Before(tick.Date) {

		bar.Volume = bar.Volume + tick.Volume
		if bar.Low > tick.Price {
			bar.Low = tick.Price
		}
		if bar.High < tick.Price {
			bar.High = tick.Price
		}
		bar.Close = tick.Price
		(*bars)[count-1] = bar
	} else if bar.Date.Truncate(time.Second).Before(tick.Date) {
		*bars = append(*bars, newChart)
	}
}

func BenchmarkName(b *testing.B) {
	f, err := os.Open("./ticksample.csv")
	require.NoError(b, err)
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	require.NoError(b, err)

	charts := &[]model.Chart{}
	for i := 0; i < b.N; i++ {
		for _, i := range records[1:] {

			price, err := strconv.ParseFloat(i[1], 64)
			assert.NoError(b, err, i[1])

			vol, err := strconv.ParseFloat(i[5], 64)
			assert.NoError(b, err, i[5])
			ti, err := time.Parse("2006-01-02T15:04:05.999", i[0])
			assert.NoError(b, err, i[0])
			tick := model.Tick{
				Code:   "none",
				Date:   ti,
				Price:  price,
				Volume: vol,
			}

			MakeBar(charts, tick)
		}
	}
}

func Test_Resampling(t *testing.T) {
	f, err := os.Open("./ticksample.csv")
	require.NoError(t, err)
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	require.NoError(t, err)

	charts := &[]model.Chart{}

	for _, i := range records[1:] {
		price, err := strconv.ParseFloat(i[1], 64)
		assert.NoError(t, err, i[1])

		vol, err := strconv.ParseFloat(i[5], 64)
		assert.NoError(t, err, i[5])
		ti, err := time.Parse("2006-01-02T15:04:05.999", i[0])
		assert.NoError(t, err, i[0])
		tick := model.Tick{
			Code:   "none",
			Date:   ti,
			Price:  price,
			Volume: vol,
		}

		MakeBar(charts, tick)
	}

	for _, i := range *charts {
		fmt.Println(i.Close)
		fmt.Println(i.Open)
		fmt.Println(i.High)
		fmt.Println(i.Low)
		fmt.Println(i.Volume)
		fmt.Println(i.Date.Format(time.RFC3339))
	}

}
