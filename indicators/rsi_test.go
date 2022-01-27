/*
 *  Copyright 2021 The Trader Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
package indicators

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gobenpark/trader/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRsi(t *testing.T) {

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

	rsi := NewRsi(14)
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

	//rsi.Calculate(c)
	for _, i := range rsi.Get() {
		fmt.Println(i)
	}
}

func TestCandleBuffer(t *testing.T) {
	buf := container.NewCandleBuffer([]container.Candle{
		{
			Code:   "005930",
			Open:   79500,
			High:   79600,
			Low:    78600,
			Close:  78900,
			Volume: 11000502,
			Date:   time.UnixMilli(1641945600000),
		},
		{
			Code:   "005930",
			Open:   79300,
			High:   79300,
			Low:    77900,
			Close:  77900,
			Volume: 13889401,
			Date:   time.UnixMilli(1642032000000),
		},
		{
			Code:   "005930",
			Open:   77700,
			High:   78100,
			Low:    77100,
			Close:  77300,
			Volume: 10096725,
			Date:   time.UnixMilli(1642118400000),
		},
		{
			Code:   "005930",
			Open:   77600,
			High:   77800,
			Low:    76900,
			Close:  77500,
			Volume: 8785122,
			Date:   time.UnixMilli(1642377600000),
		},
		{
			Code:   "005930",
			Open:   77600,
			High:   77800,
			Low:    76600,
			Close:  77000,
			Volume: 9592788,
			Date:   time.UnixMilli(1642464000000),
		},
		{
			Code:   "005930",
			Open:   76500,
			High:   76900,
			Low:    76100,
			Close:  76300,
			Volume: 10598290,
			Date:   time.UnixMilli(1642550400000),
		},
		{
			Code:   "005930",
			Open:   76200,
			High:   76700,
			Low:    75900,
			Close:  76500,
			Volume: 9708168,
			Date:   time.UnixMilli(1642636800000),
		},
		{
			Code:   "005930",
			Open:   75800,
			High:   75800,
			Low:    74700,
			Close:  75600,
			Volume: 15774880,
			Date:   time.UnixMilli(1642723200000),
		},
		{
			Code:   "005930",
			Open:   75400,
			High:   75800,
			Low:    74700,
			Close:  75100,
			Volume: 13691134,
			Date:   time.UnixMilli(1642982400000),
		},
		{
			Code:   "005930",
			Open:   74800,
			High:   75000,
			Low:    73200,
			Close:  74000,
			Volume: 17511258,
			Date:   time.Now(),
		},
	})

	require.Equal(t, buf.Len(), 10)

	rsi := NewRsi(9)
	buff := make([]container.Candle, 10)

	fmt.Println(buf.Read(buff))
	fmt.Println(buff)

	rsi.Calculate(buff)
	fmt.Println(rsi.Get())

}
