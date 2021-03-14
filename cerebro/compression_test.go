/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */

package cerebro

import (
	"fmt"
	"testing"
	"time"

	"github.com/gobenpark/trader/domain"
	"github.com/stretchr/testify/assert"
)

func TestCompression(t *testing.T) {
	ti1, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:00:01")
	ti2, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:01:01")
	ti3, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:01:02")
	ti4, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:02:02")
	ti5, _ := time.Parse("2006-01-02 15:04:05", "2021-03-12 00:03:02")
	input := []domain.Tick{
		{
			Code:   "test",
			Date:   ti1,
			Price:  10,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti2,
			Price:  10,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti3,
			Price:  10,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti4,
			Price:  10,
			Volume: 10,
		},
		{
			Code:   "test",
			Date:   ti5,
			Price:  10,
			Volume: 10,
		},
	}
	ch := make(chan domain.Tick)
	go func() {
		defer close(ch)
		for _, i := range input {
			ch <- i
		}
	}()

	result := []domain.Candle{}
	for d := range Compression(ch, time.Minute) {
		result = append(result, d)
	}
	assert.Len(t, result, 3)

	for _, i := range result {
		fmt.Println(i)
	}
}
