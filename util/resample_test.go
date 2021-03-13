package util

import (
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/store/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Compression(tick <-chan domain.Tick, level time.Duration) <-chan domain.Candle {
	ch := make(chan domain.Candle, 1)
	go func() {
		defer close(ch)
		c := domain.Candle{}
		for t := range tick {
			if c.Date.Equal(time.Time{}) || t.Date.After(c.Date) {
				if !reflect.DeepEqual(c, domain.Candle{}) {
					ch <- c
				}
				rounded := t.Date.Round(level)
				if t.Date.Sub(rounded) > 0 {
					rounded = rounded.Add(level)
				}

				c.Date = rounded
			}
			c.Volume += t.Volume
			c.Code = t.Code
			c.Close = t.Price
			if c.Open == 0 {
				c.Open = t.Price
			}

			if c.High < t.Price {
				c.High = t.Price
			}

			if c.Low == 0 || c.Low > t.Price {
				c.Low = t.Price
			}
		}
	}()
	return ch
}

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
func roundUpTime(t time.Time, roundOn time.Duration) time.Time {
	t = t.Round(roundOn)

	if time.Since(t) >= 0 {
		t = t.Add(roundOn)
	}

	return t
}

func TestTime(t *testing.T) {
	f, err := os.Open("ticksample.csv")
	require.NoError(t, err)
	r := csv.NewReader(f)
	data, err := r.ReadAll()
	require.NoError(t, err)

	ch := make(chan domain.Tick)
	go func() {
		for i := range Compression(ch, 0) {
			fmt.Printf("%#v\n", i)
		}
	}()
	for _, i := range data[1:] {
		ti, err := time.Parse("2006-01-02T15:04:05.999", i[0])
		assert.NoError(t, err)
		p, err := strconv.ParseFloat(i[4], 64)
		assert.NoError(t, err)
		v, err := strconv.ParseFloat(i[5], 64)
		assert.NoError(t, err)

		tick := domain.Tick{
			Code:   "KRW-BTC",
			Date:   ti,
			Price:  p,
			Volume: v,
		}
		ch <- tick
	}
}
