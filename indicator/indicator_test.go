package indicator

import (
	"fmt"
	"testing"
	"time"
)

func TestMean(t *testing.T) {
	tk := make(chan Tick, 1)
	sg := NewValue(nil)
	sg.Start(tk)
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
	sg.Start(tk)

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

		sg.Start(tk)
	}()

	for i := range value.value {
		fmt.Println("roi", i.Value, "raw data", i.Tick)
	}
}

func TestFilter(t *testing.T) {
	tk := make(chan Tick, 1)
	sg := NewValue(nil)
	sg.Start(tk)

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

func TestCombineF(t *testing.T) {
	tk := make(chan Tick, 1)
	sg := NewValue(nil)
	sg.Start(tk)

	v := sg

	go func() {
		var initvalue int64 = 0
	Done:
		for {
			time.Sleep(500 * time.Millisecond)
			if initvalue%2 == 0 {
				tk <- Tick{AskBid: "ask", Price: initvalue, Volume: initvalue * 3, Date: time.Now()}
			} else {
				tk <- Tick{AskBid: "bid", Price: initvalue, Volume: initvalue * 3, Date: time.Now()}
			}
			initvalue++

			if initvalue == 100 {
				close(tk)
				break Done
			}
		}
	}()

	v.Copy().Price().Transaction(func(v Packet) {
		fmt.Println("price", v)
	})

	CombineWithF(time.Minute, func(v ...float64) float64 {
		return v[0]
	}, v.Copy().Price().Filter(func(value Packet) bool {
		return value.Value > 0.5
	}), v.Copy().Volume().ROI(time.Minute).Filter(func(value Packet) bool {
		return value.Value > 0.5
	})).Transaction(func(v Packet) {
		fmt.Println(v.Tick)

	})

	CombineWithF(time.Minute, func(v ...float64) float64 {
		return v[0]
	}, v.Copy().Price().ROI(30*time.Second).Filter(func(value Packet) bool {
		return value.Value < 0.5
	}), v.Copy().Volume().ROI(30*time.Second).Filter(func(value Packet) bool {
		return value.Value < 0.5
	})).Transaction(func(v Packet) {
		fmt.Println(v)
	})

	time.Sleep(time.Hour)

}
