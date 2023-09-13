/*
 * Copyright 2023 The Trader Authors
 *
 * Licensed under the GNU General Public License v3.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   <https:fsf.org/>
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package strategy

import (
	"fmt"
	"testing"
	"time"

	"github.com/reactivex/rxgo/v2"
)

func TestRxGO_Stream(t *testing.T) {
	ch := make(chan rxgo.Item)

	go func() {
		i := 0
		for {
			time.Sleep(500 * time.Millisecond)
			fmt.Println(i)
			ch <- rxgo.Of(i)
			i += 1
		}
	}()

	ob := rxgo.FromEventSource(ch).BufferWithTime(rxgo.WithDuration(3 * time.Second))

	go func() {
		for i := range ob.Observe() {
			fmt.Printf("first data: %d, now: %s\n", i, time.Now().Format("2006-01-02 15:04:05"))
		}
	}()

	go func() {
		for i := range ob.BufferWithTime(rxgo.WithDuration(5 * time.Second)).Observe() {

			fmt.Printf("second data: %d, now: %s\n", i, time.Now().Format("2006-01-02 15:04:05"))
		}
	}()

	go func() {
		fmt.Printf("count")
		fmt.Println(ob.Count().Get())
		for i := range ob.Count().Observe() {
			fmt.Printf("avg: %d\n", i)
		}
	}()

	time.Sleep(time.Hour)
}

func TestRule(t *testing.T) {

}
