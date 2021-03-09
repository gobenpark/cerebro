package feeds

import (
	"fmt"

	"github.com/gobenpark/trader/domain"
	"github.com/looplab/fsm"
	"github.com/rs/zerolog/log"
)

type UpbitFeed struct {
	DefaultFeed
	fsm  *fsm.FSM
	Code string
}

func NewUpbitFeed(code string) domain.Feed {
	d := &UpbitFeed{
		Code: code,
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

func (u *UpbitFeed) HistoryLoading(e *fsm.Event) {

}

func (u *UpbitFeed) Start(history, isLive bool) {
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
