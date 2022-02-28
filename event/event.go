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

//go:generate mockgen -source=./event.go -destination=./mock/mock_event.go

package event

type EventType int

const (
	Buy EventType = iota + 1
	Sell
	Cash
)

type Listener interface {
	Listen(e interface{})
}

type Broadcaster interface {
	BroadCast(e interface{})
}

type Event struct {
	EventType EventType
	Message   string
	Value     interface{}
}

type OrderEvent struct {
	Event
	State string
	Oid   string
}
