// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"slices"
)

// RoomListFilter decides whether a room list entry belongs to a section of the room list.
// Mirrors RoomListFilter in the web frontend's statestore.
type RoomListFilter interface {
	ID() string
	Name() string
	Include(entry *RoomListEntry) bool
}

// SimpleRoomListFilter is a RoomListFilter backed by a function.
type SimpleRoomListFilter struct {
	FilterID   string
	FilterName string
	Func       func(entry *RoomListEntry) bool
	// Sort optionally reorders rooms within a group. Rooms that compare equal keep their room list order.
	Sort SortFunc
}

var _ RoomListFilter = (*SimpleRoomListFilter)(nil)

func (f *SimpleRoomListFilter) ID() string                        { return f.FilterID }
func (f *SimpleRoomListFilter) Name() string                      { return f.FilterName }
func (f *SimpleRoomListFilter) Include(entry *RoomListEntry) bool { return f.Func(entry) }

var (
	InvitesFilter = &SimpleRoomListFilter{
		FilterID:   "fi.mau.gomuks.invites",
		FilterName: "Invites",
		Func:       func(entry *RoomListEntry) bool { return entry.IsInvite },
	}
	FavoritesFilter = &SimpleRoomListFilter{
		FilterID:   "m.favourite",
		FilterName: "Favourites",
		Func:       (*RoomListEntry).IsFavorite,
		Sort:       FavoriteSort,
	}
	LowPriorityFilter = &SimpleRoomListFilter{
		FilterID:   "m.lowpriority",
		FilterName: "Low priority",
		Func:       (*RoomListEntry).IsLowPriority,
	}
	OtherRoomsFilter = &SimpleRoomListFilter{
		FilterID:   "fi.mau.gomuks.rooms",
		FilterName: "Rooms",
		Func:       func(entry *RoomListEntry) bool { return true },
	}
)

// DefaultTagGroups is the default list of room list groups when grouping rooms by tag.
var DefaultTagGroups = []RoomListFilter{InvitesFilter, FavoritesFilter, LowPriorityFilter, OtherRoomsFilter}

// RoomListGroup is a section of the room list.
type RoomListGroup struct {
	Filter RoomListFilter
	Rooms  []*RoomListEntry
	// UnreadRooms is the number of rooms in the group that have unread messages or are marked as unread.
	UnreadRooms int
	// Highlights is true if any room in the group has unread highlights.
	Highlights bool
}

// HasActivity returns true if the room has unread messages, highlights or was marked as unread.
func (entry *RoomListEntry) HasActivity() bool {
	return entry.UnreadMessages > 0 || entry.UnreadHighlights > 0 || entry.MarkedUnread
}

// GroupRoomList splits a room list into groups. Each room is put in the first group whose filter includes it,
// rooms that don't match any filter are dropped. The rooms in each group keep their order from the input list
// (which is expected to be in display order, i.e. ReversedRoomList), unless the filter defines a sort function.
// Empty groups are included in the output.
func GroupRoomList(rooms []*RoomListEntry, filters []RoomListFilter) []*RoomListGroup {
	groups := make([]*RoomListGroup, len(filters))
	for i, filter := range filters {
		groups[i] = &RoomListGroup{Filter: filter}
	}
	for _, room := range rooms {
		for _, group := range groups {
			if group.Filter.Include(room) {
				group.Rooms = append(group.Rooms, room)
				if room.HasActivity() {
					group.UnreadRooms++
				}
				if room.UnreadHighlights > 0 {
					group.Highlights = true
				}
				break
			}
		}
	}
	for _, group := range groups {
		if simple, ok := group.Filter.(*SimpleRoomListFilter); ok && simple.Sort != nil {
			// The input is in display order (highest first), so sort in descending order.
			slices.SortStableFunc(group.Rooms, func(a, b *RoomListEntry) int {
				return simple.Sort(b, a)
			})
		}
	}
	return groups
}
