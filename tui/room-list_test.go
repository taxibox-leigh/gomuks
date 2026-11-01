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
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/hicli/jsoncmd"
	"go.mau.fi/gomuks/pkg/rpc/client"
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
)

func newTestRoomList(t *testing.T, grouped bool, metas ...*database.Room) *RoomList {
	t.Helper()
	gs := store.NewStore()
	sync := &jsoncmd.SyncComplete{Rooms: make(map[id.RoomID]*jsoncmd.SyncRoom)}
	for _, meta := range metas {
		sync.Rooms[meta.ID] = &jsoncmd.SyncRoom{Meta: meta}
	}
	gs.ApplySync(sync)
	view := &MainView{
		matrix: &client.GomuksClient{GomuksStore: gs},
		config: &config.Config{RoomListGroupByTag: grouped},
	}
	return NewRoomList(view)
}

func testMeta(name string, ts int64, unread, highlights int, tags event.Tags) *database.Room {
	return &database.Room{
		ID:               id.RoomID("!" + name),
		Name:             new(name),
		SortingTimestamp: jsontime.UM(time.UnixMilli(ts)),
		Tags:             tags,
		UnreadCounts:     database.UnreadCounts{UnreadMessages: unread, UnreadHighlights: highlights},
	}
}

func drawRoomList(list *RoomList, width, height int) []string {
	screen := tcell.NewSimulationScreen("UTF-8")
	_ = screen.Init()
	screen.SetSize(width, height)
	list.Draw(mauview.NewProxyScreen(screen, 0, 0, width, height))
	screen.Show()
	cells, w, h := screen.GetContents()
	lines := make([]string, h)
	for y := range h {
		var line strings.Builder
		for x := range w {
			cell := cells[y*w+x]
			if len(cell.Runes) > 0 {
				line.WriteRune(cell.Runes[0])
			} else {
				line.WriteRune(' ')
			}
		}
		lines[y] = strings.TrimRight(line.String(), " ")
	}
	return lines
}

func TestRoomListDraw(t *testing.T) {
	low := event.Tags{event.RoomTagLowPriority: {}}
	fav := event.Tags{event.RoomTagFavourite: {Order: "0.5"}}
	metas := []*database.Room{
		testMeta("Alice", 50, 2, 0, nil),
		testMeta("Bridge bot", 40, 0, 0, low),
		testMeta("Team chat", 30, 0, 0, fav),
		testMeta("Some very long room name", 20, 120, 1, low),
		testMeta("Bob", 10, 0, 0, nil),
	}

	t.Run("ungrouped is unchanged", func(t *testing.T) {
		got := drawRoomList(newTestRoomList(t, false, metas...), 25, 6)
		want := []string{
			"Alice                 (2)",
			"Bridge bot",
			"Team chat",
			"Some very long room(120!)",
			"Bob",
			"",
		}
		if !slices.Equal(want, got) {
			t.Errorf("want:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
		}
	})

	t.Run("grouped and collapsed", func(t *testing.T) {
		list := newTestRoomList(t, true, metas...)
		want := []string{
			"▼ Favourites 1",
			"Team chat",
			"▼ Low priority 2",
			"Bridge bot",
			"Some very long room(120!)",
			"▼ Rooms 2",
			"Alice                 (2)",
			"Bob",
		}
		if got := drawRoomList(list, 25, 8); !slices.Equal(want, got) {
			t.Errorf("want:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
		}

		list.SetSelected("!Bridge bot")
		list.ToggleSelectedGroup()
		want = []string{
			"▼ Favourites 1",
			"Team chat",
			"▶ Low priority 2     (1!)",
			"Bridge bot",
			"▼ Rooms 2",
			"Alice                 (2)",
			"Bob",
			"",
		}
		if got := drawRoomList(list, 25, 8); !slices.Equal(want, got) {
			t.Errorf("want:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
		}
		if next := list.Next(); next != "!Alice" {
			t.Errorf("Next() from peeked room: want !Alice, got %s", next)
		}
		if next := list.NextWithActivity(); next != "!Some very long room name" {
			t.Errorf("NextWithActivity(): collapsed group with a highlight should be included, got %s", next)
		}

		list.ToggleAllGroups()
		want = []string{
			"▶ Favourites 1",
			"▶ Low priority 2     (1!)",
			"Bridge bot",
			"▶ Rooms 2             (1)",
			"",
		}
		if got := drawRoomList(list, 25, 5); !slices.Equal(want, got) {
			t.Errorf("want:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
		}
	})

	t.Run("narrow header truncates name first", func(t *testing.T) {
		list := newTestRoomList(t, true, metas...)
		drawRoomList(list, 14, 5)
		list.SetSelected("!Bridge bot")
		list.ToggleSelectedGroup()
		got := drawRoomList(list, 14, 5)[2]
		if want := "▶ Low … 2 (1!)"; got != want {
			t.Errorf("want %q, got %q", want, got)
		}
	})
}
