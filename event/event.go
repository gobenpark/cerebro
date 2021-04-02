/*                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
 */

//go:generate mockgen -source=./event.go -destination=./mock/mock_event.go

package event

type Listener interface {
	Listen(e interface{})
}

type Broadcaster interface {
	BroadCast(e interface{})
}

type Event struct {
	EventType string
	Message   string
}

type OrderEvent struct {
	Event
	State string
	Oid   string
}
