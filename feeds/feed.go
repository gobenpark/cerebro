package feeds

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gobenpark/trader/domain"
	"github.com/looplab/fsm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	HISTORYBACK = "history_back"
	LIVE        = "live"
	LOAD        = "load"
	IDLE        = "idle"
)

type FeedEngine struct {
	fsm    *fsm.FSM
	Code   string
	Strats []domain.Strategy
	log    zerolog.Logger
	store  domain.Store
}

func NewFeed(code string, api domain.Store) domain.Feed {
	d := &FeedEngine{
		Code: code,
		log: zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
			With().Timestamp().Str("logger", "feed").Logger(),
		store: api,
	}
	d.fsm = fsm.NewFSM(
		IDLE,
		fsm.Events{
			{Name: "historyloading", Src: []string{HISTORYBACK}, Dst: LOAD},
			{Name: "finish", Src: []string{HISTORYBACK, LOAD, IDLE, LIVE}, Dst: IDLE},
			{Name: "liveloading", Src: []string{LIVE}, Dst: LOAD},
			{Name: "readylive", Src: []string{IDLE}, Dst: LIVE},
			{Name: "loading", Src: []string{LOAD}, Dst: LOAD},
			{Name: "history", Src: []string{IDLE}, Dst: HISTORYBACK},
		},
		fsm.Callbacks{
			"enter_state": d.stateStart,
			LOAD: func(event *fsm.Event) {
				fmt.Println(event)
			},
			"finish": func(event *fsm.Event) {
				fmt.Println(event)
			},
			"liveloading":    d.loadTick,
			"historyloading": d.HistoryLoading,
		},
	)
	return d
}

func (u *FeedEngine) stateStart(e *fsm.Event) {
	u.log.Info().Msgf("state change from %s to %s\n", e.Src, e.Dst)
}

func (u *FeedEngine) HistoryLoading(e *fsm.Event) {
	c, err := u.store.LoadHistory(context.Background(), u.Code)
	if err != nil {
		log.Err(err).Send()
		return
	}
	for _, i := range c {
		fmt.Println(i)
	}
}

func (u *FeedEngine) Start(history, isLive bool, strats []domain.Strategy) {
	u.Strats = strats
	if history {
		u.fsm.Event("history")
		if err := u.fsm.Event("historyloading"); err != nil {
			log.Err(err).Send()
		}
		u.fsm.Event("finish")
	}

	if isLive {
		u.fsm.Event("readylive")
		u.fsm.Event("liveloading")
		u.fsm.Event("finish")
	}
}

func (u *FeedEngine) loadTick(e *fsm.Event) {

	t, err := u.store.LoadTick(context.Background(), u.Code)
	if err != nil {
		log.Err(err).Send()
		return
	}

	for i := range t {
		fmt.Println(i)
	}
}

func (u *FeedEngine) AddStore(store domain.Store) {
	u.store = store
}
