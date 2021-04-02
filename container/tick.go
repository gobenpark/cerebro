/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
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
