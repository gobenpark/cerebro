/*
 * Copyright 2023 The Trader Authors
 *
 * Licensed under the GNU General Public License v3.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   <https:fsf.org/>
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package container

import (
	"context"
	"fmt"
	"testing"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/stretchr/testify/require"
)

func TickDatas(t *testing.T) []Tick {
	t.Helper()
	cli := influxdb2.NewClient("http://192.168.0.58:8086", "-wrQo-soyW1A-xl4ROPAIU0jMx-1SEeoAZJYQSmcfUpQuA5cZyNmJeqD3AA8wfZGggG2v7fG5AW_M0nv2ADGQQ==")
	fmt.Println(cli.Ping(context.TODO()))
	api := cli.QueryAPI("stock")
	re, err := api.Query(context.TODO(), `
import "join"

price = from(bucket: "stock")
|> range(start: 2023-04-01, stop: now())
|> filter(fn: (r) => r._field == "price")
|> filter(fn: (r) => r.code == "005930")
|> group(columns: ["_time","_value","_field"],mode: "except")


volume = from(bucket: "stock")
|> range(start: 2023-04-01, stop: now())
|> filter(fn: (r) => r._field == "volume")
|> filter(fn: (r) => r.code == "005930")
|> group(columns: ["_time","_value","_field"],mode: "except")


join.time(
left: price,
right: volume,
as: (l,r) => ({l with volume: r._value})
)
`)
	require.NoError(t, err)

	tks := []Tick{}

	for re.Next() {
		tk := Tick{
			Code:   re.Record().ValueByKey("code").(string),
			Date:   re.Record().ValueByKey("_time").(time.Time),
			Price:  re.Record().ValueByKey("_value").(int64),
			Volume: re.Record().ValueByKey("volume").(int64),
		}
		tks = append(tks, tk)
	}

	return tks
}
