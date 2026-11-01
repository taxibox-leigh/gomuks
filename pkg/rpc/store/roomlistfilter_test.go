// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"slices"
	"testing"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
)

func TestGroupRoomList(t *testing.T) {
	// Display order, most recent first.
	rooms := []*RoomListEntry{
		{RoomID: "!invite", IsInvite: true, UnreadCounts: database.UnreadCounts{UnreadHighlights: 1}},
		{RoomID: "!plain1", UnreadCounts: database.UnreadCounts{UnreadMessages: 2}},
		{RoomID: "!fav-late", Tags: event.Tags{event.RoomTagFavourite: {Order: "0.9"}}},
		{RoomID: "!both", Tags: event.Tags{event.RoomTagFavourite: {}, event.RoomTagLowPriority: {}}},
		{RoomID: "!low1", Tags: lowTag, UnreadCounts: database.UnreadCounts{UnreadMessages: 5}},
		{RoomID: "!fav-early", Tags: event.Tags{event.RoomTagFavourite: {Order: "0.1"}}},
		{RoomID: "!low2", Tags: lowTag, MarkedUnread: true},
		{RoomID: "!plain2"},
		{RoomID: "!low3", Tags: lowTag, UnreadCounts: database.UnreadCounts{UnreadHighlights: 1, UnreadMessages: 1}},
	}
	groups := GroupRoomList(rooms, DefaultTagGroups)
	want := []struct {
		id          string
		rooms       []id.RoomID
		unreadRooms int
		highlights  bool
	}{
		{"fi.mau.gomuks.invites", []id.RoomID{"!invite"}, 1, true},
		// Ordered by tag order, an entry without an order counts as 0.
		{"m.favourite", []id.RoomID{"!both", "!fav-early", "!fav-late"}, 0, false},
		{"m.lowpriority", []id.RoomID{"!low1", "!low2", "!low3"}, 3, true},
		{"fi.mau.gomuks.rooms", []id.RoomID{"!plain1", "!plain2"}, 1, false},
	}
	if len(groups) != len(want) {
		t.Fatalf("want %d groups, got %d", len(want), len(groups))
	}
	for i, w := range want {
		g := groups[i]
		if g.Filter.ID() != w.id {
			t.Errorf("group %d: want ID %s, got %s", i, w.id, g.Filter.ID())
		}
		if got := roomIDs(g.Rooms); !slices.Equal(w.rooms, got) {
			t.Errorf("group %s: want rooms %v, got %v", w.id, w.rooms, got)
		}
		if g.UnreadRooms != w.unreadRooms || g.Highlights != w.highlights {
			t.Errorf("group %s: want unread %d/%v, got %d/%v", w.id, w.unreadRooms, w.highlights, g.UnreadRooms, g.Highlights)
		}
	}
}

func TestGroupRoomListEmpty(t *testing.T) {
	groups := GroupRoomList(nil, DefaultTagGroups)
	if len(groups) != len(DefaultTagGroups) {
		t.Fatalf("want %d groups, got %d", len(DefaultTagGroups), len(groups))
	}
	for _, g := range groups {
		if len(g.Rooms) != 0 {
			t.Errorf("group %s should be empty", g.Filter.ID())
		}
	}
}
