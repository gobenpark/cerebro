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

package broker

import "errors"

var (
	ErrNotEnoughCash       = errors.New("Not Enough Cash")
	ErrPositionExists      = errors.New("Broker Position is Exists")
	ErrPositionNotExists   = errors.New("Broker Position is not Exists")
	ErrLowSizeThenPosition = errors.New("Low Size then position")
)
