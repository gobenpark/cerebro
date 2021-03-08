package store

import (
	"context"
	"fmt"
	"github.com/gobenpark/trader/store/model"
	"testing"
)

func TestAlpaSquare_day(t *testing.T) {
	store := &AlpaSquare{charts: make(chan model.Chart, 1000)}
	go store.day(context.Background(), "005930")
	for i := range store.charts {
		fmt.Println(i)
	}
}

func TestAlphaStream(t *testing.T) {
	store := &AlpaSquare{charts: make(chan model.Chart, 1)}
	go store.TickStream(context.Background())

	for i := range store.charts {
		//fmt.Println(i.Date)
		fmt.Println(i.Date.String())
	}
}

func TestFilename(t *testing.T) {
	txt := "workspace/201209052608477Y23L/210118014710021WHjE/진짜긴파일명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명명.txt"
	fmt.Println(len(txt))
}
