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
package analysis

import (
	"context"
	"fmt"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
)

type Analyzer interface {
	event.Listener
	Next(tk indicator.Tick)
}

type Engine struct {
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Spawn(ctx context.Context, item []item.Item, tk <-chan indicator.Tick) error {
	for i := range tk {
		fmt.Println("analyzer", i)
	}
	return nil
}

func (e *Engine) Listen(i interface{}) {
}
