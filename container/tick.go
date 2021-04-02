/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */
package container

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
