// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/hicli/jsoncmd"
)

var (
	favTag = event.Tags{event.RoomTagFavourite: {Order: "0.5"}}
	lowTag = event.Tags{event.RoomTagLowPriority: {}}
)

func testRoomMeta(roomID id.RoomID, ts int64, tags event.Tags) *database.Room {
	return &database.Room{
		ID:               roomID,
		Name:             new(string(roomID)),
		SortingTimestamp: jsontime.UM(time.UnixMilli(ts)),
		Tags:             tags,
		UnreadCounts:     database.UnreadCounts{UnreadMessages: 1},
	}
}

func roomIDs(entries []*RoomListEntry) []id.RoomID {
	out := make([]id.RoomID, len(entries))
	for i, entry := range entries {
		out[i] = entry.RoomID
	}
	return out
}

func syncRooms(metas ...*database.Room) *jsoncmd.SyncComplete {
	sync := &jsoncmd.SyncComplete{Rooms: make(map[id.RoomID]*jsoncmd.SyncRoom)}
	for _, meta := range metas {
		sync.Rooms[meta.ID] = &jsoncmd.SyncRoom{Meta: meta}
	}
	return sync
}

func syncPrefs(t *testing.T, prefs Preferences) *jsoncmd.SyncComplete {
	t.Helper()
	content, err := json.Marshal(prefs)
	if err != nil {
		t.Fatal(err)
	}
	return &jsoncmd.SyncComplete{AccountData: map[event.Type]*database.AccountData{
		AccountDataGomuksPreferences: {Type: AccountDataGomuksPreferences.Type, Content: content},
	}}
}

func assertOrder(t *testing.T, gs *GomuksStore, want ...id.RoomID) {
	t.Helper()
	got := roomIDs(gs.ReversedRoomList.Current())
	if !slices.Equal(want, got) {
		t.Errorf("room list order: want %v, got %v", want, got)
	}
}

func TestFavoriteOrder(t *testing.T) {
	tests := []struct {
		tags      event.Tags
		wantOrder float64
		wantFav   bool
	}{
		{nil, 0, false},
		{lowTag, 0, false},
		{favTag, 0.5, true},
		{event.Tags{event.RoomTagFavourite: {}}, 0, true},
		{event.Tags{event.RoomTagFavourite: {Order: "garbage"}}, 0, true},
	}
	for _, tt := range tests {
		order, fav := (&RoomListEntry{Tags: tt.tags}).FavoriteOrder()
		if order != tt.wantOrder || fav != tt.wantFav {
			t.Errorf("FavoriteOrder(%v) = %v, %v; want %v, %v", tt.tags, order, fav, tt.wantOrder, tt.wantFav)
		}
	}
}

func TestRoomListSortFunc(t *testing.T) {
	entries := []*RoomListEntry{
		{RoomID: "!low-new", SortingTimestamp: time.UnixMilli(50), Tags: lowTag},
		{RoomID: "!fav2", SortingTimestamp: time.UnixMilli(10), Tags: event.Tags{event.RoomTagFavourite: {Order: "0.9"}}},
		{RoomID: "!new", SortingTimestamp: time.UnixMilli(40)},
		{RoomID: "!fav1", SortingTimestamp: time.UnixMilli(5), Tags: event.Tags{event.RoomTagFavourite: {Order: "0.1"}}},
		{RoomID: "!old", SortingTimestamp: time.UnixMilli(1)},
	}
	tests := []struct {
		name  string
		prefs Preferences
		want  []id.RoomID // top of the list first
	}{
		{"timestamp", Preferences{}, []id.RoomID{"!low-new", "!new", "!fav2", "!fav1", "!old"}},
		{"pin favorites", Preferences{PinFavorites: true}, []id.RoomID{"!fav1", "!fav2", "!low-new", "!new", "!old"}},
		{"pin low priority", Preferences{PinLowPriority: true}, []id.RoomID{"!new", "!fav2", "!fav1", "!old", "!low-new"}},
		{"pin both", Preferences{PinFavorites: true, PinLowPriority: true}, []id.RoomID{"!fav1", "!fav2", "!new", "!old", "!low-new"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, _ := RoomListSortFunc(&tt.prefs)
			sorted := slices.Clone(entries)
			slices.SortStableFunc(sorted, fn)
			slices.Reverse(sorted)
			if got := roomIDs(sorted); !slices.Equal(tt.want, got) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestApplySyncTags(t *testing.T) {
	gs := NewStore()
	gs.ApplySync(syncRooms(
		testRoomMeta("!a", 10, nil),
		testRoomMeta("!b", 20, nil),
		testRoomMeta("!c", 30, lowTag),
	))
	assertOrder(t, gs, "!c", "!b", "!a")

	// A tag-only meta change must update the room list entry.
	gs.ApplySync(syncRooms(testRoomMeta("!a", 10, favTag)))
	entry := gs.ReversedRoomList.Current()[2]
	if !entry.IsFavorite() {
		t.Fatalf("expected !a to be a favourite after tag change, got tags %v", entry.Tags)
	}

	// Pinning favourites and low priority re-sorts the whole list.
	gs.ApplySync(syncPrefs(t, Preferences{PinFavorites: true, PinLowPriority: true}))
	assertOrder(t, gs, "!a", "!b", "!c")

	// Incremental updates respect the pinned sort.
	gs.ApplySync(syncRooms(testRoomMeta("!d", 40, nil)))
	assertOrder(t, gs, "!a", "!d", "!b", "!c")

	// Removing the favourite tag moves the room back into the timestamp-sorted part.
	gs.ApplySync(syncRooms(testRoomMeta("!a", 10, nil)))
	assertOrder(t, gs, "!d", "!b", "!a", "!c")
}

func TestApplySyncMuteLowPriority(t *testing.T) {
	gs := NewStore()
	gs.ApplySync(syncRooms(testRoomMeta("!low", 10, lowTag), testRoomMeta("!normal", 20, nil)))
	unreads := func() map[id.RoomID]int {
		out := make(map[id.RoomID]int)
		for _, entry := range gs.ReversedRoomList.Current() {
			out[entry.RoomID] = entry.UnreadMessages
		}
		return out
	}
	if got := unreads(); got["!low"] != 1 || got["!normal"] != 1 {
		t.Fatalf("unexpected unreads before muting: %v", got)
	}
	gs.ApplySync(syncPrefs(t, Preferences{MuteLowPriority: true}))
	if got := unreads(); got["!low"] != 0 || got["!normal"] != 1 {
		t.Fatalf("unexpected unreads after muting: %v", got)
	}
}
