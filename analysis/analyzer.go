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
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/broker"
	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/store"
)

type Analyzer interface {
	event.Listener
}

type Engine struct {
}

func NewEngine(log log.Logger, bk *broker.Broker, preload bool, store store.Store, cache *badger.DB, timeout time.Duration) *Engine {
	return &Engine{}
}

func (a *Engine) Spawn(ctx context.Context, tk <-chan indicator.Tick, item []item.Item) error {

}
