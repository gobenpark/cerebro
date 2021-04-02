/*
 * Copyright (c) 2021. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * Morbi non lorem porttitor neque feugiat blandit. Ut vitae ipsum eget quam lacinia accumsan.
 * Etiam sed turpis ac ipsum condimentum fringilla. Maecenas magna.
 * Proin dapibus sapien vel ante. Aliquam erat volutpat. Pellentesque sagittis ligula eget metus.
 * Vestibulum commodo. Ut rhoncus gravida arcu.
 */
package retry

import "time"

func Retry(max int, f func() error) error {
	retries := 0
	for {
		if err := f(); err != nil {
			<-time.After((1 << retries) * time.Second)
			retries++

			if retries >= max {
				return err
			}
			continue
		}
		return nil
	}
}
