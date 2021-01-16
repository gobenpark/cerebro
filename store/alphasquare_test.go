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
