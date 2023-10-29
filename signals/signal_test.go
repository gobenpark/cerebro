package signals

import (
	"fmt"
	"testing"
	"time"

	"github.com/gobenpark/cerebro/indicator"
)

func TestMean(t *testing.T) {
	tk := make(chan indicator.Tick, 1)
	sg := NewSignal(make(chan indicator.Tick, 1))

	value := sg.Volume().Mean(3 * time.Second)

	go func() {
		var initvalue int64 = 0
	Done:
		for {
			time.Sleep(500 * time.Millisecond)
			tk <- indicator.Tick{Volume: initvalue}
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
	tk := make(chan indicator.Tick, 1)
	sg := NewSignal(tk)

	value := sg.Volume().ROI(6 * time.Second)

	go func() {
		var initvalue int64 = 100
	Done:
		for {
			time.Sleep(500 * time.Millisecond)
			tk <- indicator.Tick{Volume: initvalue}
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
