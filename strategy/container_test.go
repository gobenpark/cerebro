package strategy

import (
	"fmt"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/indicator"
)

func TestTick(t *testing.T) {
	container := &container{
		Code:  "005930",
		Store: nil,
		cache: &badger.DB{},
	}

	container.UpdateTick(indicator.Tick{
		DiffRate:  10,
		Code:      "005930",
		AskBid:    "ask",
		Date:      time.Now(),
		Price:     10,
		AccVolume: 10,
		Volume:    10,
	})
}

func TestQueue(t *testing.T) {
	queue := make(chan int, 2)

	queue <- 0
	queue <- 10

	select {
	case queue <- 10:
	default:
		fmt.Println("queue is full")
	}

	fmt.Println(cap(queue))
	fmt.Println(len(queue))
}
