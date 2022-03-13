/*
 *  Copyright 2021 The Trader Authors
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

package main

import (
	"testing"

	"github.com/gobenpark/trader/cerebro"
	mock_store "github.com/gobenpark/trader/store/mock"
	"github.com/golang/mock/gomock"
)

func TestTrade(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mock_store.NewMockStore(ctrl)

	items := store.GetMarketItems()
	var codes []string
	for _, code := range items {
		codes = append(codes, code.Code)
	}

	c := cerebro.NewCerebro(
		cerebro.WithLive(),
		cerebro.WithStore(store),
		cerebro.WithTargetItem(codes...),
	)
	c.SetStrategy(st{})
	c.Start()
}
