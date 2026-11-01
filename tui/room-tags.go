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
	"context"
	"encoding/json"

	"maunium.net/go/mautrix/event"

	"go.mau.fi/gomuks/pkg/hicli/jsoncmd"
	"go.mau.fi/gomuks/tui/debug"
)

const lowPriorityTag = event.RoomTagLowPriority

// toggleTagContent returns new m.tag content with the given tag added or removed. Other tags and their metadata
// are preserved as-is.
func toggleTagContent(content json.RawMessage, tag event.RoomTag) (newContent json.RawMessage, tagged bool, err error) {
	var parsed struct {
		Tags map[event.RoomTag]json.RawMessage `json:"tags"`
	}
	if len(content) > 0 {
		// Malformed existing content is replaced rather than blocking the toggle.
		_ = json.Unmarshal(content, &parsed)
	}
	if parsed.Tags == nil {
		parsed.Tags = make(map[event.RoomTag]json.RawMessage)
	}
	if _, ok := parsed.Tags[tag]; ok {
		delete(parsed.Tags, tag)
	} else {
		parsed.Tags[tag] = json.RawMessage("{}")
		tagged = true
	}
	newContent, err = json.Marshal(&parsed)
	return
}

// ToggleCurrentRoomTag adds or removes a tag on the current room.
func (view *MainView) ToggleCurrentRoomTag(tag event.RoomTag) {
	if view.currentRoom == nil {
		return
	}
	room := view.currentRoom.Room
	var oldContent json.RawMessage
	if ad := room.GetAccountData(event.AccountDataRoomTags); ad != nil {
		oldContent = ad.Content
	}
	newContent, tagged, err := toggleTagContent(oldContent, tag)
	if err != nil {
		debug.Print("Failed to build tag content for", room.ID, err)
		return
	}
	go func() {
		defer debug.Recover()
		err := view.matrix.SetAccountData(context.TODO(), &jsoncmd.SetAccountDataParams{
			RoomID:  room.ID,
			Type:    event.AccountDataRoomTags.Type,
			Content: newContent,
		})
		if err != nil {
			debug.Print("Failed to set tag", tag, "=", tagged, "in", room.ID, err)
		}
	}()
}
