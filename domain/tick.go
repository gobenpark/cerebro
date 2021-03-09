package domain

import (
	"encoding/json"
	"time"
)

type Tick struct {
	Code   string    `json:"code"`
	Date   time.Time `json:"date"`
	Price  float64   `json:"price"`
	Volume float64   `json:"volume"`
}

func (t *Tick) UnmarshalJSON(bytes []byte) error {
	var data map[string]interface{}
	err := json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}
	t.Code = data["code"].(string)
	ti, err := time.Parse("2006-01-02T15:04:05", data["dt"].(string))
	if err != nil {
		return err
	}

	t.Date = ti
	t.Price = data["price"].(float64)
	t.Volume = data["volume"].(float64)
	return nil
}
