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

// container package is store of tick data or candle stick data
package container

import "context"

type ControlPlane struct {
	buffer []Tick
}

func NewControlPlane() *ControlPlane {
	return &ControlPlane{}
}

func (d *ControlPlane) Add(tick Tick) {
	d.buffer = append(d.buffer, tick)
}

func (d *ControlPlane) calculate() {

}

func (d *ControlPlane) merge(ctx context.Context, ch <-chan Tick) {
	for i := range ch {

	}
}
