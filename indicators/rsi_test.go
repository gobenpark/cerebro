/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
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
