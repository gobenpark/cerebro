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
	"context"
	"testing"
	"time"

	"github.com/gobenpark/trader/analysis"
	"github.com/gobenpark/trader/cerebro"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	mock_store "github.com/gobenpark/trader/store/mock"
	"github.com/golang/mock/gomock"
)

func TestTrade(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mock_store.NewMockStore(ctrl)

	sto := NewStore()

	store.EXPECT().Tick(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, codes ...string) (<-chan container.Tick, error) {
		return sto.Tick(ctx, codes...)
	}).AnyTimes()

	store.
		EXPECT().
		Order(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, o order.Order) error {
			//return errors.New("거절")
			return nil
		}).AnyTimes()

	store.EXPECT().Candles(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, code string, level time.Duration) (container.Candles, error) {
		return sto.Candles(ctx, code, level)
	}).AnyTimes()

	store.
		EXPECT().
		Positions().
		Return(map[string]position.Position{}).
		AnyTimes()

	store.EXPECT().Cash().Return(int64(400000)).AnyTimes()

	items := sto.GetMarketItems()
	var codes []string
	for _, code := range items {
		codes = append(codes, code.Code)
	}

	c := cerebro.NewCerebro(
		cerebro.WithLive(),
		cerebro.WithStore(store),
		cerebro.WithTargetItem(codes...),
		cerebro.WithAnalyzer(analysis.NewInmemoryAnalyzer()),
		cerebro.WithCommision(0.05),
		cerebro.WithPreload(true),
	)

	c.SetStrategy(st{})
	c.Start()
}
