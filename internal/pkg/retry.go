/*
 *  Copyright 2021 The Cerebro Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
package pkg

import (
	"context"
	"time"
)

func Retry(ctx context.Context, max int, f func() error) error {
	retries := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := f(); err != nil {
				<-time.After((1 << retries) * time.Second)
				retries++

				if retries >= max {
					return err
				}
				continue
			}
		}
		return nil
	}
}
