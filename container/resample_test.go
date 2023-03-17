package container

import (
	"context"
	"fmt"
	"testing"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/stretchr/testify/require"
)

func Test_Resample(t *testing.T) {
	cli := influxdb2.NewClient("http://192.168.0.58:8086", "k4CT-8Fp41VunltxFJmhckBUvMCjNECybU8JM6Nzh2Uqlc1DtyC1NYjWKjpi9wl4T9MUCf461TLNtii7jDZw9g==")
	fmt.Println(cli.Ping(context.TODO()))
	api := cli.QueryAPI("stock")
	re, err := api.Query(context.TODO(), `
import "join"

price = from(bucket: "stock")
|> range(start: 2022-01-01, stop: now())
|> filter(fn: (r) => r._field == "price")
|> filter(fn: (r) => r.code == "005930")
|> group(columns: ["_time","_value","_field"],mode: "except")

volume = from(bucket: "stock")
|> range(start: 2022-01-01, stop: now())
|> filter(fn: (r) => r._field == "volume")
|> filter(fn: (r) => r.code == "005930")
|> group(columns: ["_time","_value","_field"],mode: "except")


join.time(
left: price,
right: volume,
as: (l,r) => ({l with volume: r._value})
)

`)

	target := time.Minute
	require.NoError(t, err)

	tks := []*Candle{}

	for re.Next() {
		tk := Tick{
			Code:   re.Record().ValueByKey("code").(string),
			Date:   re.Record().ValueByKey("_time").(time.Time),
			Price:  re.Record().ValueByKey("_value").(int64),
			Volume: re.Record().ValueByKey("volume").(int64),
		}

		if len(tks) == 0 {
			tks = append(tks, &Candle{
				Open:  tk.Price,
				High:  tk.Price,
				Low:   tk.Price,
				Close: tk.Price,
				Date:  tk.Date.Truncate(target),
			})
			continue
		}

		cd := tks[len(tks)-1]

		edge := cd.Date.Add(target).Truncate(target)

		if tk.Date.Before(edge) {
			cd.Close = tk.Price

			if cd.Low > tk.Price {
				cd.Low = tk.Price
			}

			if cd.High < tk.Price {
				cd.High = tk.Price
			}
		} else {
			tks = append(tks, &Candle{
				Open:  tk.Price,
				High:  tk.Price,
				Low:   tk.Price,
				Close: tk.Price,
				Date:  tk.Date.Truncate(time.Minute),
			})
		}
	}
	for _, i := range tks {
		fmt.Printf("%#v\n", i)
	}
}

func TestTime(t *testing.T) {
	now, _ := time.Parse("15:04:05", "09:15:01")
	fmt.Println(now.Truncate(15 * time.Minute))

}
