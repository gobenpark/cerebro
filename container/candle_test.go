package container

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuffer(t *testing.T) {
	buf := bytes.NewBuffer([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.Equal(t, buf.Len(), 9)
	fmt.Println(buf.Cap())

	fmt.Println(buf.Next(1))
	fmt.Println(buf.Next(1))
	fmt.Println(buf.Cap())
	fmt.Println(buf.Len())
}

func TestBufferSwap(t *testing.T) {
	buf := NewCandleBuffer([]Candle{
		{
			Date: time.Now(),
		},
		{
			Date: time.Now().Add(time.Minute),
		},
		{
			Date: time.Now().Add(2 * time.Minute),
		},
	})

	for _, i := range buf.buf {
		fmt.Println(i.Date)
	}

	fmt.Println()
	sort.Sort(sort.Reverse(buf))
	for _, i := range buf.buf {
		fmt.Println(i.Date)
	}
}
