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

// Retry calls f up to attempts times with exponential backoff between tries.
// It returns nil on the first success, the last error after all attempts fail,
// or ctx.Err() if the context is canceled while waiting to retry.
func Retry(ctx context.Context, attempts int, f func() error) error {
	var err error
	for retries := range attempts {
		if err = f(); err == nil {
			return nil
		}

		// Wait for the backoff, but stay responsive to cancellation.
		timer := time.NewTimer((1 << retries) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}
