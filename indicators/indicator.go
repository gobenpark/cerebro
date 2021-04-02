/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */
package indicators

import (
	"time"

	"github.com/gobenpark/trader/container"
)

type Indicator interface {
	Calculate(container container.Container)
	Get() []Indicate
}

type Indicate struct {
	Data float64
	Date time.Time
}
