/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */
package container

import "time"

type Candle struct {
	Code   string    `json:"code" validate:"required"`
	Open   float64   `json:"open" validate:"required"`
	High   float64   `json:"high" validate:"required"`
	Low    float64   `json:"low" validate:"required"`
	Close  float64   `json:"close" validate:"required"`
	Volume float64   `json:"volume" validate:"required"`
	Date   time.Time `json:"date" validate:"required"`
}
