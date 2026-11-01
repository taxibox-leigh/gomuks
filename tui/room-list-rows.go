// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tui

import (
	"slices"

	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/rpc/store"
)

// roomListRow is one line of the room list: either a group header or a room.
type roomListRow struct {
	group *store.RoomListGroup
	room  *store.RoomListEntry
	// collapsed is set on header rows of collapsed groups.
	collapsed bool
}

// buildRoomListRows flattens the room list into rows. Without groups, every room is a row. With groups, each
// non-empty group gets a header row followed by its rooms, unless the group is collapsed. The selected room is
// still shown under the header of a collapsed group so that it stays visible.
func buildRoomListRows(rooms []*store.RoomListEntry, groups []*store.RoomListGroup, collapsed map[string]bool, selected id.RoomID) []roomListRow {
	if groups == nil {
		rows := make([]roomListRow, len(rooms))
		for i, room := range rooms {
			rows[i] = roomListRow{room: room}
		}
		return rows
	}
	rows := make([]roomListRow, 0, len(rooms)+len(groups))
	for _, group := range groups {
		if len(group.Rooms) == 0 {
			continue
		}
		isCollapsed := collapsed[group.Filter.ID()]
		rows = append(rows, roomListRow{group: group, collapsed: isCollapsed})
		if !isCollapsed {
			for _, room := range group.Rooms {
				rows = append(rows, roomListRow{room: room})
			}
		} else if selected != "" {
			idx := slices.IndexFunc(group.Rooms, func(room *store.RoomListEntry) bool {
				return room.RoomID == selected
			})
			if idx != -1 {
				rows = append(rows, roomListRow{room: group.Rooms[idx]})
			}
		}
	}
	return rows
}

func roomRowIndex(rows []roomListRow, roomID id.RoomID) int {
	if roomID == "" {
		return -1
	}
	return slices.IndexFunc(rows, func(row roomListRow) bool {
		return row.room != nil && row.room.RoomID == roomID
	})
}

// adjacentRoom returns the room in the next (dir > 0) or previous (dir < 0) room row from the selected one,
// skipping group headers. If nothing is selected, moving forward returns the first room.
func adjacentRoom(rows []roomListRow, selected id.RoomID, dir int) id.RoomID {
	idx := roomRowIndex(rows, selected)
	if idx == -1 {
		if dir < 0 || selected != "" {
			return ""
		}
	}
	for i := idx + dir; i >= 0 && i < len(rows); i += dir {
		if rows[i].room != nil {
			return rows[i].room.RoomID
		}
	}
	return ""
}

// nextRoomWithActivity returns the first room with unread activity, skipping invites (which can't be opened as a
// room) and the selected room. Rooms in collapsed groups are only considered if they have highlights.
func nextRoomWithActivity(rooms []*store.RoomListEntry, groups []*store.RoomListGroup, collapsed map[string]bool, selected id.RoomID) id.RoomID {
	if groups == nil {
		groups = []*store.RoomListGroup{{Rooms: rooms}}
	}
	for _, group := range groups {
		isCollapsed := group.Filter != nil && collapsed[group.Filter.ID()]
		for _, room := range group.Rooms {
			if room.IsInvite || room.RoomID == selected || (isCollapsed && room.UnreadHighlights == 0) {
				continue
			}
			if room.HasActivity() {
				return room.RoomID
			}
		}
	}
	return ""
}

// groupOfRoom returns the group that contains the given room.
func groupOfRoom(groups []*store.RoomListGroup, roomID id.RoomID) *store.RoomListGroup {
	for _, group := range groups {
		if slices.ContainsFunc(group.Rooms, func(room *store.RoomListEntry) bool {
			return room.RoomID == roomID
		}) {
			return group
		}
	}
	return nil
}
