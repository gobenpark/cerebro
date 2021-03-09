package feeds

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gobenpark/proto/stock"
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

type UpbitFeed struct {
	Data   map[string]map[time.Duration][]domain.Candle `json:"data"`
	client stock.StockClient
	fsm    *fsm.FSM
	Code   string
	Strats []domain.Strategy
	log    zerolog.Logger
}

func NewUpbitFeed(code string, client stock.StockClient) domain.Feed {
	d := &UpbitFeed{
		Code:   code,
		client: client,
		Data:   map[string]map[time.Duration][]domain.Candle{},
		log: zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
			With().Timestamp().Str("logger", "feed").Logger(),
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

func (u *UpbitFeed) stateStart(e *fsm.Event) {
	u.log.Info().Msgf("state change from %s to %s\n", e.Src, e.Dst)
}

func (u *UpbitFeed) HistoryLoading(e *fsm.Event) {
	charts, err := u.client.Chart(context.Background(), &stock.ChartRequest{
		Code: u.Code,
		To:   nil,
	})
	if err != nil {
		e.Cancel(err)
	}
	for _, i := range charts.GetData() {
		fmt.Println(i)
		for _, s := range u.Strats {
			s.Next(u.Data)
		}
	}

}

func (u *UpbitFeed) Start(history, isLive bool, strats []domain.Strategy) {
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

func (u *UpbitFeed) loadTick(e *fsm.Event) {

}
