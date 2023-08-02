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

package indicators

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/gobenpark/cerebro/container"
	"github.com/stretchr/testify/require"

	_ "embed"
)

//go:embed candle.json
var data []byte

func Test_BollingerBand(t *testing.T) {
	var candles container.Candles
	err := json.Unmarshal(data, &candles)
	require.NoError(t, err)

	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Date.Before(candles[j].Date)
	})

	m, top, b := BollingerBand(20, candles)
	for k := range top {
		fmt.Println(m[k].Data, top[k].Data, b[k].Data)
	}

	_, _ = top, b

}
