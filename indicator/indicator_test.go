package indicator

import (
	"fmt"
	"testing"
	"time"
)

func TestMean(t *testing.T) {
	tk := make(chan Tick, 1)
	sg := NewValue(nil)

	value := sg.Volume().Mean(3 * time.Second)

	go func() {
		var initvalue int64 = 0
	Done:
		for {
			time.Sleep(500 * time.Millisecond)
			tk <- Tick{Volume: initvalue}
			initvalue++
			if initvalue == 10 {
				close(tk)
				break Done
			}
		}
	}()

	for i := range value.value {
		fmt.Println("mean", i)
	}
}

func TestROI(t *testing.T) {
	tk := make(chan Tick, 1)
	sg := NewValue(nil)

	value := sg.Volume().ROI(6 * time.Second)

	go func() {
		var initvalue int64 = 100
	Done:
		for {
			time.Sleep(500 * time.Millisecond)
			tk <- Tick{Volume: initvalue}
			initvalue -= 2
			if initvalue == 0 {
				close(tk)
				break Done
			}
		}
	}()

	for i := range value.value {
		fmt.Println("roi", i)
	}
}

func TestFilter(t *testing.T) {
	tk := make(chan Tick, 1)
	sg := NewValue(nil)

	data := sg.Filter(func(tick Tick) bool {
		return tick.AskBid == "ask"
	})

	value := data.Price()

	vvalue := data.Volume()

	go func() {
		var initvalue int64 = 0
	Done:
		for {
			time.Sleep(500 * time.Millisecond)
			if initvalue%2 == 0 {
				tk <- Tick{AskBid: "ask", Price: initvalue, Volume: initvalue * 2}
			} else {
				tk <- Tick{AskBid: "bid", Price: initvalue, Volume: initvalue * 2}
			}
			initvalue++

			if initvalue == 100 {
				close(tk)
				break Done
			}
		}
	}()

	go func() {
		for i := range value.value {
			fmt.Println("price", i)
		}
	}()
	go func() {
		for i := range vvalue.value {
			fmt.Println("volume", i)
		}
	}()
	time.Sleep(time.Hour)
}
