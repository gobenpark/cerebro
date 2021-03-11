package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gobenpark/trader/store/model"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

func init() {
	log.Logger = log.With().Caller().Logger()
}

type AlpaSquare struct {
	charts chan model.Chart
}

func NewAlpaSquareStore() *AlpaSquare {
	return &AlpaSquare{charts: make(chan model.Chart, 1000)}
}

func (a *AlpaSquare) day(ctx context.Context, code string) error {
	defer close(a.charts)
	u := url.URL{
		Scheme:   "https",
		Host:     "api.alphasquare.co.kr",
		Path:     "/api/square/chart/" + code,
		RawQuery: "freq=day",
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	cli := http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: 10 * time.Second,
		},
		Timeout: 10 * time.Second,
	}

	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var data map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return err
	}

	var charts [][]interface{}
	if err := json.Unmarshal(data["data"], &charts); err != nil {
		return err
	}

	for _, i := range charts {
		result := model.Chart{
			Code:   code,
			Low:    i[3].(float64),
			High:   i[2].(float64),
			Open:   i[1].(float64),
			Close:  i[4].(float64),
			Volume: i[5].(float64),
			//Date:   i[0].(string),
		}
		a.charts <- result
	}

	return nil
}

func (a *AlpaSquare) TickStream(ctx context.Context) {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.alphasquare.co.kr/socketio/socket.io/?EIO=3&transport=websocket", nil)
	if err != nil {
		log.Err(err).Send()
	}

	err = c.WriteMessage(websocket.BinaryMessage, []byte("42[\"authenticate\", null]"))
	if err != nil {
		log.Err(err).Send()
	}

	data := []interface{}{"subscribe_real"}
	data = append(data, map[string][]string{"codes": {"005930", "035420", "005380"}})

	bt, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}

	result := fmt.Sprintf("42%s", string(bt))
	err = c.WriteMessage(websocket.BinaryMessage, []byte(result))
	if err != nil {
		fmt.Println(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				break
			case <-time.After(3 * time.Second):
				err := c.WriteMessage(websocket.BinaryMessage, []byte("2"))
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}()

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Err(err).Send()
				return
			}
			var res []interface{}
			if len(message) > len("0{\"sid\":\"ec6f5ebce6d042e1bad1cd19e6179260\",\"upgrades\":[],\"pingTimeout\":10000,\"pingInterval\":5000}") {
				err = json.Unmarshal(message[2:], &res)
				if err != nil {
					fmt.Println("jsonerror: ", err)
				}

				if result, ok := res[1].(map[string]interface{}); ok {
					data := result["data"].(string)
					var tick model.Tick
					err := json.Unmarshal([]byte(data), &tick)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Println(tick)
				}
			}
		}
	}()
}

func (a *AlpaSquare) Start(ctx context.Context) {
	a.day(ctx, "005930")
}

func (a *AlpaSquare) Data() <-chan model.Chart {
	return a.charts
}
