package main

import (
	"context"
	"crypto/sha512"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/form"
	"github.com/go-resty/resty/v2"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/item"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Account struct {
	Currency            string `json:"currency"`
	Balance             string `json:"balance"`
	Locked              string `json:"locked"`
	AvgBuyPrice         string `json:"avg_buy_price"`
	AvgBuyPriceModified bool   `json:"avg_buy_price_modified"`
	UnitCurrency        string `json:"unit_currency"`
}

type Order struct {
	Market     string `json:"market" form:"market"`
	Side       string `json:"side" form:"side"`
	Volume     string `json:"volume" form:"volume"`
	Price      string `json:"price" form:"price"`
	OrdType    string `json:"ordType" form:"ord_type"`
	Identifier string `json:"identifier" form:"identifier"`
}

type UpbitOrder struct {
	Uuid            string        `json:"uuid"`
	Side            string        `json:"side"`
	OrdType         string        `json:"ord_type"`
	Price           string        `json:"price"`
	State           string        `json:"state"`
	Market          string        `json:"market"`
	CreatedAt       time.Time     `json:"created_at"`
	Volume          string        `json:"volume"`
	RemainingVolume string        `json:"remaining_volume"`
	ReservedFee     string        `json:"reserved_fee"`
	RemainingFee    string        `json:"remaining_fee"`
	PaidFee         string        `json:"paid_fee"`
	Locked          string        `json:"locked"`
	ExecutedVolume  string        `json:"executed_volume"`
	TradesCount     int           `json:"trades_count"`
	Trades          []interface{} `json:"trades"`
}
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

//go:embed secretkey
var secretkey []byte

//go:embed accesskey
var accesskey []byte

func (u Upbit) CreateToken() error {

	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"access_key": string(accesskey),
		"nonce":      uuid.NewV4().String(),
	})

	key, err := tk.SignedString(secretkey)
	if err != nil {
		return err
	}

	u.Client.SetAuthToken(key)
	u.Client.SetAuthScheme("Bearer")

	return nil
}

func (u Upbit) OrderToken(values url.Values) (string, error) {
	sha := sha512.New()
	sha.Write([]byte(values.Encode()))
	h2 := sha.Sum(nil)

	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"access_key":     string(accesskey),
		"query_hash":     fmt.Sprintf("%x", h2),
		"query_hash_alg": "SHA512",
		"nonce":          uuid.NewV4().String(),
	})

	return tk.SignedString(secretkey)
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

func (u Upbit) Candles(ctx context.Context, code string, level time.Duration) (container.Candles, error) {
	var res *resty.Response
	var err error
	min := int(level / time.Minute)
	switch min {
	case 1, 3, 5, 15, 10, 30, 60, 240:
		res, err = u.Client.R().
			SetQueryParam("market", code).
			SetQueryParam("count", "200").
			Get(fmt.Sprintf("/candles/minutes/%d", min))
	case 1440:
		res, err = u.Client.R().
			SetQueryParam("market", code).
			SetQueryParam("count", "200").
			Get("/candles/days")
	case 1440 * 7:
		res, err = u.Client.R().
			SetQueryParam("market", code).
			SetQueryParam("count", "200").
			Get("/candles/weeks")
	default:
		return nil, fmt.Errorf("not found candle types: %d", min)
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
	c, _, err := websocket.DefaultDialer.DialContext(ctx, "wss://api.upbit.com/websocket/v1", nil)
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

func (u *Upbit) Order(ctx context.Context, o order.Order) error {

	switch o.Action() {
	case order.Buy:
		od := Order{
			Market: o.Code(),
			Side:   "bid",
			Volume: "",
			Price:  fmt.Sprintf("%f", o.OrderPrice()),
			OrdType: func() string {
				switch o.Exec() {
				case order.Limit:
					return "limit"
				case order.Market:
					return "price"
				default:
					return ""
				}
			}(),
			Identifier: o.ID(),
		}

		values, err := form.NewEncoder().Encode(od)
		if err != nil {
			return err
		}

		token, err := u.OrderToken(values)
		if err != nil {
			return err
		}
		res, err := u.Client.SetDebug(true).R().
			SetQueryString(values.Encode()).
			SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
			SetBody(od).
			Post("/orders")
		if err != nil {
			return err
		}

		fmt.Println(string(res.Body()))

	case order.Sell:
		od := Order{
			Market: o.Code(),
			Side:   "ask",
			Volume: fmt.Sprintf("%d", o.Size()),
			Price:  fmt.Sprintf("%f", o.Price()),
			OrdType: func() string {
				switch o.Exec() {
				case order.Limit:
					return "limit"
				case order.Market:
					return "market"
				default:
					return ""
				}
			}(),
			Identifier: o.ID(),
		}

		values, err := form.NewEncoder().Encode(od)
		if err != nil {
			return err
		}

		token, err := u.OrderToken(values)
		if err != nil {
			return err
		}
		res, err := u.Client.R().
			SetQueryString(values.Encode()).
			SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
			Post("/orders")
		if err != nil {
			return err
		}

		fmt.Println(string(res.Body()))
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.position[o.Code()] = position.Position{
		Code:      o.Code(),
		Size:      o.Size(),
		Price:     o.Price(),
		CreatedAt: time.Now(),
	}

	return nil
}

func (u *Upbit) Cancel(id string) error {
	values := url.Values{}
	values.Add("identifier", id)
	token, err := u.OrderToken(values)
	if err != nil {
		return err
	}

	res, err := u.Client.R().
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetQueryString(values.Encode()).
		Delete("/order")
	if err != nil {
		return err
	}
	fmt.Println(string(res.Body()))
	return nil
}

func (Upbit) LoadHistory(ctx context.Context, code string, d time.Duration) ([]container.Candle, error) {
	panic("implement me")
}

func (Upbit) Uid() string {
	panic("implement me")
}

func (u *Upbit) accounts() ([]Account, error) {
	err := u.CreateToken()
	if err != nil {
		fmt.Println(err)
	}

	res, err := u.Client.R().Get("/accounts")
	if err != nil {
		return nil, err
	}
	var data []Account

	if err := json.Unmarshal(res.Body(), &data); err != nil {
		panic(err)
	}

	return data, nil
}

func (u *Upbit) Cash() int64 {

	data, err := u.accounts()
	if err != nil {
		fmt.Println(err)
		return 0
	}
	for _, i := range data {
		if i.Currency == "KRW" {
			fmt.Println("this")
			cu, err := strconv.ParseFloat(i.Balance, 10)
			if err != nil {
				fmt.Println(err)
			}
			return int64(cu)
		}
	}
	return 0
}

func (Upbit) Commission() float64 {
	return 0.05
}

func (u *Upbit) Positions() map[string]position.Position {
	data, err := u.accounts()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	result := map[string]position.Position{}
	for _, i := range data {
		result[fmt.Sprintf("KRW-%s", i.Currency)] = position.Position{
			Code: fmt.Sprintf("KRW-%s", i.Currency),
			Size: func() int64 {
				cu, err := strconv.ParseFloat(i.Balance, 10)
				if err != nil {
					fmt.Println(err)
				}
				return int64(cu)
			}(),
			Price: func() float64 {
				cu, err := strconv.ParseFloat(i.AvgBuyPrice, 10)
				if err != nil {
					fmt.Println(err)
				}
				return cu
			}(),
			CreatedAt: time.Time{},
		}
	}

	return result
}

//func (Upbit) OrderState(ctx context.Context) (<-chan event.OrderEvent, error) {
//	panic("implement me")
//}

func (u *Upbit) OrderInfo(id string) (order.Order, error) {

	values := url.Values{}
	values.Add("identifier", id)
	token, err := u.OrderToken(values)
	if err != nil {
		return nil, err
	}

	u.Client.SetAuthToken(token)
	res, err := u.Client.R().
		SetQueryString(values.Encode()).
		Get("/order")
	if err != nil {
		return nil, err
	}

	var data UpbitOrder
	err = json.Unmarshal(res.Body(), &data)
	if err != nil {
		return nil, err
	}
	fmt.Println(data)
	return nil, nil
}
