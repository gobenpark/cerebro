package container

import (
	"fmt"
	"testing"
	"time"
)

func TestResample(t *testing.T) {

	source := Candles{{
		Open:   10,
		High:   20,
		Low:    30,
		Close:  40,
		Volume: 50,
		Date:   time.Now().Truncate(time.Second),
	},
	}

	target := Candles{
		{
			Open:   10,
			High:   9,
			Low:    2,
			Close:  20,
			Volume: 30,
			Date:   time.Now().Add(time.Minute).Truncate(time.Second),
		},
		{
			Open:   20,
			High:  10,
			Low:    1,
			Close:  10,
			Volume: 20,
			Date:   time.Now().Add(time.Minute).Truncate(time.Second),
		},
	}

	result := CalculateCandle(source,time.Minute,target)
	fmt.Printf("%#v",result)
}
