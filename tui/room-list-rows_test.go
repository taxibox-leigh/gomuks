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
	"testing"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/rpc/store"
)

var testRooms = []*store.RoomListEntry{
	{RoomID: "!invite", IsInvite: true, UnreadCounts: database.UnreadCounts{UnreadHighlights: 1}},
	{RoomID: "!plain1"},
	{RoomID: "!fav", Tags: event.Tags{event.RoomTagFavourite: {}}},
	{RoomID: "!low1", Tags: event.Tags{event.RoomTagLowPriority: {}}, UnreadCounts: database.UnreadCounts{UnreadMessages: 3}},
	{RoomID: "!low2", Tags: event.Tags{event.RoomTagLowPriority: {}}},
	{RoomID: "!plain2", UnreadCounts: database.UnreadCounts{UnreadMessages: 1}},
}

// rowNames renders rows as "# group" for headers ("> group" when collapsed) and the room ID for rooms.
func rowNames(rows []roomListRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		switch {
		case row.group != nil && row.collapsed:
			out[i] = "> " + row.group.Filter.ID()
		case row.group != nil:
			out[i] = "# " + row.group.Filter.ID()
		default:
			out[i] = string(row.room.RoomID)
		}
	}
	return out
}

func TestBuildRoomListRows(t *testing.T) {
	groups := store.GroupRoomList(testRooms, store.DefaultTagGroups)
	collapsed := map[string]bool{"m.lowpriority": true}
	tests := []struct {
		name     string
		groups   []*store.RoomListGroup
		selected id.RoomID
		want     []string
	}{
		{"ungrouped", nil, "", []string{"!invite", "!plain1", "!fav", "!low1", "!low2", "!plain2"}},
		{"grouped", groups, "!plain1", []string{
			"# fi.mau.gomuks.invites", "!invite",
			"# m.favourite", "!fav",
			"> m.lowpriority",
			"# fi.mau.gomuks.rooms", "!plain1", "!plain2",
		}},
		{"selected room in collapsed group stays visible", groups, "!low2", []string{
			"# fi.mau.gomuks.invites", "!invite",
			"# m.favourite", "!fav",
			"> m.lowpriority", "!low2",
			"# fi.mau.gomuks.rooms", "!plain1", "!plain2",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rowNames(buildRoomListRows(testRooms, tt.groups, collapsed, tt.selected))
			if !slices.Equal(tt.want, got) {
				t.Errorf("want %v\ngot  %v", tt.want, got)
			}
		})
	}
}

func TestBuildRoomListRowsSkipsEmptyGroups(t *testing.T) {
	rooms := []*store.RoomListEntry{{RoomID: "!plain"}}
	got := rowNames(buildRoomListRows(rooms, store.GroupRoomList(rooms, store.DefaultTagGroups), nil, ""))
	want := []string{"# fi.mau.gomuks.rooms", "!plain"}
	if !slices.Equal(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestAdjacentRoom(t *testing.T) {
	groups := store.GroupRoomList(testRooms, store.DefaultTagGroups)
	collapsed := map[string]bool{"m.lowpriority": true}
	tests := []struct {
		name     string
		selected id.RoomID
		dir      int
		want     id.RoomID
	}{
		{"nothing selected, next", "", 1, "!invite"},
		{"nothing selected, previous", "", -1, ""},
		{"skips header forward", "!invite", 1, "!fav"},
		{"skips header backward", "!fav", -1, "!invite"},
		{"skips collapsed group forward", "!fav", 1, "!plain1"},
		{"skips collapsed group backward", "!plain1", -1, "!fav"},
		{"leaves peeked room forward", "!low2", 1, "!plain1"},
		{"leaves peeked room backward", "!low2", -1, "!fav"},
		{"end of list", "!plain2", 1, ""},
		{"start of list", "!invite", -1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := buildRoomListRows(testRooms, groups, collapsed, tt.selected)
			if got := adjacentRoom(rows, tt.selected, tt.dir); got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNextRoomWithActivity(t *testing.T) {
	groups := store.GroupRoomList(testRooms, store.DefaultTagGroups)
	tests := []struct {
		name      string
		groups    []*store.RoomListGroup
		collapsed map[string]bool
		selected  id.RoomID
		want      id.RoomID
	}{
		{"ungrouped skips invites", nil, nil, "", "!low1"},
		{"ungrouped skips selected room", nil, nil, "!low1", "!plain2"},
		{"grouped uses group order", groups, nil, "", "!low1"},
		{"skips collapsed groups", groups, map[string]bool{"m.lowpriority": true}, "", "!plain2"},
		{"nothing left", groups, map[string]bool{"m.lowpriority": true}, "!plain2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRoomWithActivity(testRooms, tt.groups, tt.collapsed, tt.selected); got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}

	t.Run("collapsed group with highlight", func(t *testing.T) {
		rooms := []*store.RoomListEntry{
			{RoomID: "!low", Tags: event.Tags{event.RoomTagLowPriority: {}}, UnreadCounts: database.UnreadCounts{UnreadHighlights: 1, UnreadMessages: 1}},
		}
		got := nextRoomWithActivity(rooms, store.GroupRoomList(rooms, store.DefaultTagGroups), map[string]bool{"m.lowpriority": true}, "")
		if got != "!low" {
			t.Errorf("want !low, got %q", got)
		}
	})
}
