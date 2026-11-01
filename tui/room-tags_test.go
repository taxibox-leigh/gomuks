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
	"encoding/json"
	"testing"

	"maunium.net/go/mautrix/event"
)

func TestToggleTagContent(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantJSON   string
		wantTagged bool
	}{
		{"no existing content", "", `{"tags":{"m.lowpriority":{}}}`, true},
		{"malformed content", `{`, `{"tags":{"m.lowpriority":{}}}`, true},
		{"keeps other tags", `{"tags":{"m.favourite":{"order":0.25}}}`, `{"tags":{"m.favourite":{"order":0.25},"m.lowpriority":{}}}`, true},
		{"removes tag", `{"tags":{"m.lowpriority":{"order":1},"u.work":{}}}`, `{"tags":{"u.work":{}}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, tagged, err := toggleTagContent(json.RawMessage(tt.content), event.RoomTagLowPriority)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.wantJSON || tagged != tt.wantTagged {
				t.Errorf("want %s/%v, got %s/%v", tt.wantJSON, tt.wantTagged, got, tagged)
			}
		})
	}
}
