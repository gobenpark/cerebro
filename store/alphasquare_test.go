package store

import (
	"context"
	"syscall"
	"testing"
)

func TestAlpaSquare_TickStream(t *testing.T) {
	ch := make(chan syscall.Signal)

	store := AlpaSquare{}
	store.TickStream(context.Background())
	<-ch
}
