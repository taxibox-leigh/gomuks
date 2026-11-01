// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"cmp"
	"strconv"

	"maunium.net/go/mautrix/event"
)

// FavoriteOrder returns the order of the room's m.favourite tag and whether the tag is set.
// A missing or invalid order is treated as 0, like the web frontend does.
func (entry *RoomListEntry) FavoriteOrder() (float64, bool) {
	meta, ok := entry.Tags[event.RoomTagFavourite]
	if !ok {
		return 0, false
	}
	order, _ := strconv.ParseFloat(string(meta.Order), 64)
	return order, true
}

// IsFavorite returns true if the room has the m.favourite tag.
func (entry *RoomListEntry) IsFavorite() bool {
	_, ok := entry.Tags[event.RoomTagFavourite]
	return ok
}

// IsLowPriority returns true if the room has the m.lowpriority tag.
func (entry *RoomListEntry) IsLowPriority() bool {
	_, ok := entry.Tags[event.RoomTagLowPriority]
	return ok
}

// SortFunc compares two room list entries. The room list is sorted in ascending order and displayed in reverse,
// so an entry that compares greater is displayed higher up. Mirrors SortFunc in the web frontend's statestore.
type SortFunc func(a, b *RoomListEntry) int

func TimestampSort(a, b *RoomListEntry) int {
	return a.SortingTimestamp.Compare(b.SortingTimestamp)
}

func AlphabeticalSort(a, b *RoomListEntry) int {
	return cmp.Compare(b.SearchName, a.SearchName)
}

// FavoriteSort puts favourites above other rooms, and favourites with a lower order above ones with a higher order.
func FavoriteSort(a, b *RoomListEntry) int {
	aOrder, aFav := a.FavoriteOrder()
	bOrder, bFav := b.FavoriteOrder()
	switch {
	case aFav && bFav:
		return cmp.Compare(bOrder, aOrder)
	case aFav:
		return 1
	case bFav:
		return -1
	default:
		return 0
	}
}

// LowPrioritySort puts low priority rooms below other rooms.
func LowPrioritySort(a, b *RoomListEntry) int {
	aLow, bLow := a.IsLowPriority(), b.IsLowPriority()
	switch {
	case aLow && !bLow:
		return -1
	case !aLow && bLow:
		return 1
	default:
		return 0
	}
}

// ChainedSort returns a SortFunc that uses f2 to break ties in f1.
func ChainedSort(f1, f2 SortFunc) SortFunc {
	return func(a, b *RoomListEntry) int {
		if res := f1(a, b); res != 0 {
			return res
		}
		return f2(a, b)
	}
}

// RoomListSortFunc returns the room list sort function for the given preferences,
// equivalent to the web frontend's statestore sortFunc.
func RoomListSortFunc(prefs *Preferences) (fn SortFunc, isTimestamp bool) {
	if prefs == nil {
		prefs = &DefaultPreferences
	}
	fn, isTimestamp = TimestampSort, true
	if prefs.AlphabeticalOrder {
		fn, isTimestamp = AlphabeticalSort, false
	}
	if prefs.PinFavorites {
		fn, isTimestamp = ChainedSort(FavoriteSort, fn), false
	}
	if prefs.PinLowPriority {
		fn, isTimestamp = ChainedSort(LowPrioritySort, fn), false
	}
	return
}

func roomListSortPrefsChanged(oldPrefs, newPrefs *Preferences) bool {
	if oldPrefs == nil {
		oldPrefs = &DefaultPreferences
	}
	if newPrefs == nil {
		newPrefs = &DefaultPreferences
	}
	return oldPrefs.AlphabeticalOrder != newPrefs.AlphabeticalOrder ||
		oldPrefs.PinFavorites != newPrefs.PinFavorites ||
		oldPrefs.PinLowPriority != newPrefs.PinLowPriority ||
		oldPrefs.MuteLowPriority != newPrefs.MuteLowPriority
}
