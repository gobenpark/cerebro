package feeds

import (
	"context"
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/rs/zerolog/log"
)

type FeedEngine struct {
	E chan event.Event
}

func (u *FeedEngine) HistoryLoading(code string, store domain.Store) {
	c, err := store.LoadHistory(context.Background(), code)
	if err != nil {
		log.Err(err).Send()
		return
	}
	for _, i := range c {
		fmt.Println(i)
	}
}

func (u *FeedEngine) LoadTick(code string, store domain.Store) {
	t, err := store.LoadTick(context.Background(), code)
	if err != nil {
		log.Err(err).Send()
		return
	}

	for i := range t {
		fmt.Println(i)
	}
}
