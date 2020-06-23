package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/BumwooPark/trader/store/model"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type AlpaSquare struct {
	charts chan model.Chart
}

func NewAlpaSquareStore() Storer {
	return &AlpaSquare{charts: make(chan model.Chart, 1000)}
}

func (a *AlpaSquare) day(ctx context.Context, code string) error {
	req, err := http.NewRequest(http.MethodGet, "https://api.alphasquare.co.kr/api/square/chart/"+code+"?freq=day", nil)
	if err != nil {
		return err
	}

	cli := http.Client{
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            nil,
			DialTLSContext:         nil,
			TLSClientConfig:        nil,
			TLSHandshakeTimeout:    0,
			DisableKeepAlives:      false,
			DisableCompression:     false,
			MaxIdleConns:           0,
			MaxIdleConnsPerHost:    0,
			MaxConnsPerHost:        0,
			IdleConnTimeout:        10 * time.Second,
			ResponseHeaderTimeout:  0,
			ExpectContinueTimeout:  0,
			TLSNextProto:           nil,
			ProxyConnectHeader:     nil,
			MaxResponseHeaderBytes: 0,
			WriteBufferSize:        0,
			ReadBufferSize:         0,
			ForceAttemptHTTP2:      false,
		},
		Timeout: 10 * time.Second,
	}

	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	bt, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(bt, &data); err != nil {
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
			Date:   i[0].(string),
		}
		a.charts <- result

	}

	return nil
}

func (a *AlpaSquare) TickStream(ctx context.Context) {
	c, _, err := websocket.DefaultDialer.Dial("wss://api.alphasquare.co.kr/socket.io/?EIO=3&transport=websocket", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}

	data := []interface{}{"subscribe_real"}
	data = append(data, map[string][]string{"codes": []string{"005930"}})

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
				log.Println("read:", err)
				return
			}

			//fmt.Println(string(message))
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
