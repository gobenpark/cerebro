package coin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	. "github.com/gobenpark/trader/error"

	"github.com/go-resty/resty/v2"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/item"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gobenpark/trader/store"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Upbit struct {
	mu sync.Mutex
	*resty.Client
	position  map[string]position.Position
	FirstCash int64
}

func NewStore() *Upbit {
	client := resty.New()

	client.SetHostURL("https://api.upbit.com/v1")

	return &Upbit{Client: client, position: make(map[string]position.Position), FirstCash: 90000}
}

func (u Upbit) GetMarketItems() []item.Item {
	res, err := u.Client.R().Get("/market/all?isDetails=false")
	if err != nil {
		return nil
	}

	var data []map[string]interface{}
	if err := json.Unmarshal(res.Body(), &data); err != nil {
		fmt.Println(err)
		return nil
	}

	var items []item.Item
	for _, i := range data {
		if !regexp.MustCompile("^KRW+").MatchString(i["market"].(string)) {
			continue
		}
		items = append(items, item.Item{
			Code: i["market"].(string),
			Name: i["korean_name"].(string),
			Tag: func() string {
				if i["market_warning"] != nil {
					return i["market_warning"].(string)
				}
				return ""
			}(),
		})
	}
	return items
}

func (u Upbit) Candles(ctx context.Context, code string, c store.CandleType, value int) ([]container.Candle, error) {
	var res *resty.Response
	var err error
	switch c {
	case store.MIN:
		res, err = u.Client.R().SetContext(ctx).Get(fmt.Sprintf("/candles/minutes/%d?market=%s&count=500", value, code))
	case store.DAY:
		res, err = u.Client.R().SetContext(ctx).Get(fmt.Sprintf("/candles/days?count=500&market=%s", code))
	case store.WEEK:
		res, err = u.Client.R().SetContext(ctx).Get(fmt.Sprintf("/candles/weeks?count=500&market=%s", code))
	}
	if err != nil {
		return nil, err
	}
	var candle []struct {
		Market               string  `json:"market"`
		CandleDateTimeUtc    string  `json:"candle_date_time_utc"`
		CandleDateTimeKst    string  `json:"candle_date_time_kst"`
		OpeningPrice         float64 `json:"opening_price"`
		HighPrice            float64 `json:"high_price"`
		LowPrice             float64 `json:"low_price"`
		TradePrice           float64 `json:"trade_price"`
		Timestamp            int64   `json:"timestamp"`
		CandleAccTradePrice  float64 `json:"candle_acc_trade_price"`
		CandleAccTradeVolume float64 `json:"candle_acc_trade_volume"`
		Unit                 int     `json:"unit"`
	}
	if err := json.Unmarshal(res.Body(), &candle); err != nil {
		return nil, err
	}

	var con []container.Candle
	for _, i := range candle {
		con = append(con, container.Candle{
			Code:   i.Market,
			Open:   i.OpeningPrice,
			High:   i.HighPrice,
			Low:    i.LowPrice,
			Close:  i.TradePrice,
			Volume: i.CandleAccTradeVolume,
			Date: func() time.Time {
				ti, err := time.Parse("2006-01-02T15:04:05", i.CandleDateTimeKst)
				if err != nil {
					return time.Time{}
				}
				return ti
			}(),
		})
	}
	return con, nil
}

func (u Upbit) TradeCommits(ctx context.Context, code string) ([]container.TradeHistory, error) {
	res, err := u.Client.R().Get(fmt.Sprintf("/trades/ticks?count=500&market=%s", code))
	if err != nil {
		return nil, err
	}
	var data []struct {
		Market           string  `json:"market"`
		TradeDateUtc     string  `json:"trade_date_utc"`
		TradeTimeUtc     string  `json:"trade_time_utc"`
		Timestamp        int64   `json:"timestamp"`
		TradePrice       float64 `json:"trade_price"`
		TradeVolume      float64 `json:"trade_volume"`
		PrevClosingPrice float64 `json:"prev_closing_price"`
		ChangePrice      float64 `json:"change_price"`
		AskBid           string  `json:"ask_bid"`
		SequentialId     int64   `json:"sequential_id"`
	}

	err = json.Unmarshal(res.Body(), &data)
	if err != nil {
		return nil, err
	}

	d := make([]container.TradeHistory, len(data))

	for k, v := range data {
		d[k] = container.TradeHistory{
			Code:        v.Market,
			Price:       v.TradePrice,
			Volume:      v.TradeVolume,
			PrevPrice:   v.PrevClosingPrice,
			ChangePrice: v.ChangePrice,
			ASKBID:      v.AskBid,
			Date:        time.UnixMilli(v.Timestamp),
			ID:          v.SequentialId,
		}
	}

	return d, nil
}

func (u Upbit) Tick(ctx context.Context, codes ...string) (<-chan container.Tick, error) {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.upbit.com/websocket/v1", nil)
	if err != nil {
		return nil, err
	}

	format := []map[string]interface{}{
		{
			"ticket": uuid.NewV4().String(),
		},
		{
			"type":  "ticker",
			"codes": codes,
		},
	}
	ch := make(chan container.Tick)

	bt, err := json.Marshal(format)
	if err != nil {
		return nil, err
	}

	if err := c.WriteMessage(websocket.BinaryMessage, bt); err != nil {
		log.Println(err)
		return nil, err
	}

	go func() {

		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println(message)
				return
			}

			var T struct {
				Type               string      `json:"type"`
				Code               string      `json:"code"`
				OpeningPrice       float64     `json:"opening_price"`
				HighPrice          float64     `json:"high_price"`
				LowPrice           float64     `json:"low_price"`
				TradePrice         float64     `json:"trade_price"`
				PrevClosingPrice   float64     `json:"prev_closing_price"`
				AccTradePrice      float64     `json:"acc_trade_price"`
				Change             string      `json:"change"`
				ChangePrice        float64     `json:"change_price"`
				SignedChangePrice  float64     `json:"signed_change_price"`
				ChangeRate         float64     `json:"change_rate"`
				SignedChangeRate   float64     `json:"signed_change_rate"`
				AskBid             string      `json:"ask_bid"`
				TradeVolume        float64     `json:"trade_volume"`
				AccTradeVolume     float64     `json:"acc_trade_volume"`
				TradeDate          string      `json:"trade_date"`
				TradeTime          string      `json:"trade_time"`
				TradeTimestamp     int64       `json:"trade_timestamp"`
				AccAskVolume       float64     `json:"acc_ask_volume"`
				AccBidVolume       float64     `json:"acc_bid_volume"`
				Highest52WeekPrice float64     `json:"highest_52_week_price"`
				Highest52WeekDate  string      `json:"highest_52_week_date"`
				Lowest52WeekPrice  float64     `json:"lowest_52_week_price"`
				Lowest52WeekDate   string      `json:"lowest_52_week_date"`
				TradeStatus        interface{} `json:"trade_status"`
				MarketState        string      `json:"market_state"`
				MarketStateForIos  interface{} `json:"market_state_for_ios"`
				IsTradingSuspended bool        `json:"is_trading_suspended"`
				DelistingDate      interface{} `json:"delisting_date"`
				MarketWarning      string      `json:"market_warning"`
				Timestamp          int64       `json:"timestamp"`
				AccTradePrice24H   float64     `json:"acc_trade_price_24h"`
				AccTradeVolume24H  float64     `json:"acc_trade_volume_24h"`
				StreamType         string      `json:"stream_type"`
			}
			if err := json.Unmarshal(message, &T); err != nil {
				log.Println(err)
				continue
			}

			ch <- container.Tick{
				Code:   T.Code,
				AskBid: T.AskBid,
				Date:   time.UnixMilli(T.TradeTimestamp),
				Price:  T.TradePrice,
				Volume: T.TradeVolume,
			}
		}
	}()

	go func() {
		defer close(ch)
		defer c.Close()
		ticker := time.NewTicker(time.Second * 3)
	Done:
		for {
			select {
			case <-ctx.Done():
				break Done
			case t := <-ticker.C:
				if err := c.WriteMessage(websocket.PingMessage, []byte(t.String())); err != nil {
					fmt.Println(err)
				}
			}
		}
	}()
	return ch, nil

}

func (u *Upbit) Order(ctx context.Context, o *order.Order) error {
	switch o.Action {
	case order.Buy:
		if u.FirstCash > (o.Size * int64(o.Price)) {
			u.FirstCash -= (o.Size * int64(o.Price))
		} else {
			return ErrNotEnoughMoney
		}
	case order.Sell:
		if p, ok := u.position[o.Code]; ok && p.Size == o.Size {
			u.FirstCash += (o.Size * int64(o.Price))
		} else {
			return ErrNotEnoughMoney
		}
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.position[o.Code] = position.Position{
		Code:      o.Code,
		Size:      o.Size,
		Price:     o.Price,
		CreatedAt: o.CreatedAt,
	}

	return nil
}

func (Upbit) Cancel(id string) error {
	panic("implement me")
}

func (Upbit) LoadHistory(ctx context.Context, code string, d time.Duration) ([]container.Candle, error) {
	panic("implement me")
}

func (Upbit) Uid() string {
	panic("implement me")
}

func (u *Upbit) Cash() int64 {
	return u.FirstCash
}

func (Upbit) Commission() float64 {
	panic("implement me")
}

func (u *Upbit) Positions() map[string]position.Position {
	tmap := map[string]position.Position{}
	for k, v := range u.position {
		tmap[k] = v
	}
	return tmap
}

func (Upbit) OrderState(ctx context.Context) (<-chan event.OrderEvent, error) {
	panic("implement me")
}

func (Upbit) OrderInfo(id string) (*order.Order, error) {
	panic("implement me")
}
