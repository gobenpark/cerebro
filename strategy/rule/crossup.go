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

package rule

import "github.com/gobenpark/cerebro/indicator"

type crossUp struct {
}

func NewCrossUpRule(target indicator.Indicator, base indicator.Indicator) Rule {
	return &crossUp{}
}

func (c *crossUp) Satisfied() bool {
	return false
}
