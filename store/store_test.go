package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestStore_Data(t *testing.T) {
	store := NewStore()

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	store.Start(ctx)

	go func() {
		for i := range store.Data() {
			fmt.Println(i)
		}
	}()
	time.Sleep(5 * time.Second)
}
