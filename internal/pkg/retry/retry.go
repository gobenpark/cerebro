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
