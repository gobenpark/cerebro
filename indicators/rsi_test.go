/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
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

	rsi.Calculate(c)
	for _, i := range rsi.Get() {
		fmt.Println(i)
	}
}
