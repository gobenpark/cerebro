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
	"fmt"
	"testing"

	"github.com/gobenpark/trader/order"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

func TestUpbit_Cash(t *testing.T) {
	st := NewStore()
	fmt.Println(st.Cash())
}

func TestUpbit_Positions(t *testing.T) {
	st := NewStore()
	pos := st.Positions()
	for k, v := range pos {
		fmt.Println(k)
		fmt.Println(v.Price)
		fmt.Println(v.Size)
	}
}

func TestUpbit_Order(t *testing.T) {
	st := NewStore()
	require.NoError(t, st.Order(context.TODO(), order.NewOrder("KRW-HUM", order.Buy, order.Market, 1, 10, 0.05)))
}

func TestUpbit_Sell(t *testing.T) {
	uid := uuid.NewV4().String()
	st := NewStore()
	err := st.Order(context.TODO(), order.NewOrder("KRW-JST", order.Sell, order.Limit, 100, 80.50, 0.05))
	require.NoError(t, err)
	fmt.Println(uid)

	//	{"uuid":"c131047b-79cb-4ea9-b042-024b269bfc90","side":"ask","ord_type":"limit","price":"80.5","state":"wait","market":"KRW-JST","created_at":"2022-03-13T00:07:34+09:00","volume":"100.0","remaining_volume":"100.0","reserved_fee":"0.0","remaining_fee":"0.0","paid_fee":"0.0","locked":"100.0","executed_volume":"0.0","trades_count":0}
}

func TestUpbit_OrderInfo(t *testing.T) {
	st := NewStore()
	info, err := st.OrderInfo("cca036fd-8b5b-4d2c-8bd7-6a643e0d8879")
	require.NoError(t, err)
	fmt.Println(info)
}

func TestUpbit_Cancel(t *testing.T) {
	st := NewStore()
	err := st.Cancel("cca036fd-8b5b-4d2c-8bd7-6a643e0d8879")
	require.NoError(t, err)
}
