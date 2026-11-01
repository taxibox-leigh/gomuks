// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2020 Tulir Asokan
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
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/widget"
)

type RoomList struct {
	lock sync.Mutex

	parent *MainView

	rooms    []*store.RoomListEntry
	groups   []*store.RoomListGroup
	rows     []roomListRow
	selected id.RoomID

	// grouped enables grouping rooms by tag.
	grouped bool
	// collapsed contains the IDs of collapsed groups.
	collapsed map[string]bool
	// roomListVersion is bumped whenever the store emits a new room list.
	roomListVersion atomic.Uint64
	builtVersion    uint64
	// rowsDirty is set when the rows need to be rebuilt for reasons other than a new room list.
	rowsDirty bool

	scrollOffset int
	height       int
	width        int

	// The item main text color.
	mainTextColor tcell.Color
	// The text color for selected items.
	selectedTextColor tcell.Color
	// The background color for selected items.
	selectedBackgroundColor tcell.Color
}

func NewRoomList(parent *MainView) *RoomList {
	list := &RoomList{
		parent: parent,

		scrollOffset: 0,
		rowsDirty:    true,
		collapsed:    make(map[string]bool),

		mainTextColor:           tcell.ColorDefault,
		selectedTextColor:       tcell.ColorWhite,
		selectedBackgroundColor: tcell.ColorDarkGreen,
	}
	if parent.config != nil {
		list.grouped = parent.config.RoomListGroupByTag
	}
	return list
}

// RoomListChanged marks the cached rows as outdated. It's called from the store's room list listener,
// so it must not call back into the store.
func (list *RoomList) RoomListChanged() {
	list.roomListVersion.Add(1)
}

// ensureRows rebuilds the cached rows if the room list or the grouping state changed. The lock must be held.
func (list *RoomList) ensureRows() {
	if list.parent.matrix == nil {
		return
	}
	version := list.roomListVersion.Load()
	if !list.rowsDirty && version == list.builtVersion {
		return
	}
	list.rooms = list.parent.matrix.ReversedRoomList.Current()
	if list.grouped {
		list.groups = store.GroupRoomList(list.rooms, store.DefaultTagGroups)
	} else {
		list.groups = nil
	}
	list.rows = buildRoomListRows(list.rooms, list.groups, list.collapsed, list.selected)
	list.builtVersion = version
	list.rowsDirty = false
	list.clampScrollOffset()
}

func (list *RoomList) clampScrollOffset() {
	if list.scrollOffset > len(list.rows)-list.height {
		list.scrollOffset = len(list.rows) - list.height
	}
	if list.scrollOffset < 0 {
		list.scrollOffset = 0
	}
}

func (list *RoomList) scrollToSelected() {
	pos := roomRowIndex(list.rows, list.selected)
	if pos < 0 {
		return
	}
	if pos <= list.scrollOffset {
		list.scrollOffset = pos - 1
	} else if pos >= list.scrollOffset+list.height {
		list.scrollOffset = pos - list.height + 1
	}
	list.clampScrollOffset()
}

func (list *RoomList) SetSelected(roomID id.RoomID) {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.selected = roomID
	// The selected room is shown even inside collapsed groups, so the rows may change.
	list.rowsDirty = list.rowsDirty || list.grouped
	list.ensureRows()
	list.scrollToSelected()
}

func (list *RoomList) HasSelected() bool {
	list.lock.Lock()
	defer list.lock.Unlock()
	return list.selected != ""
}

func (list *RoomList) SelectedRoom() id.RoomID {
	list.lock.Lock()
	defer list.lock.Unlock()
	return list.selected
}

func (list *RoomList) Previous() id.RoomID {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.ensureRows()
	return adjacentRoom(list.rows, list.selected, -1)
}

func (list *RoomList) Next() id.RoomID {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.ensureRows()
	return adjacentRoom(list.rows, list.selected, 1)
}

func (list *RoomList) NextWithActivity() id.RoomID {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.ensureRows()
	return nextRoomWithActivity(list.rooms, list.groups, list.collapsed, list.selected)
}

// ToggleGrouping turns grouping rooms by tag on or off for this session.
func (list *RoomList) ToggleGrouping() {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.grouped = !list.grouped
	list.rowsDirty = true
	list.ensureRows()
	list.scrollToSelected()
}

// ToggleSelectedGroup collapses the group containing the selected room, or expands it if it's already collapsed.
func (list *RoomList) ToggleSelectedGroup() {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.ensureRows()
	group := groupOfRoom(list.groups, list.selected)
	if group == nil {
		return
	}
	list.setCollapsed(group.Filter.ID(), !list.collapsed[group.Filter.ID()])
	list.scrollToSelected()
}

// ToggleAllGroups collapses all groups if any group is expanded, otherwise expands all groups.
func (list *RoomList) ToggleAllGroups() {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.ensureRows()
	anyExpanded := false
	for _, group := range list.groups {
		if len(group.Rooms) > 0 && !list.collapsed[group.Filter.ID()] {
			anyExpanded = true
			break
		}
	}
	for _, group := range list.groups {
		list.setCollapsed(group.Filter.ID(), anyExpanded)
	}
	list.scrollToSelected()
}

// setCollapsed changes the collapsed state of a group. The lock must be held.
func (list *RoomList) setCollapsed(groupID string, collapsed bool) {
	if collapsed {
		list.collapsed[groupID] = true
	} else {
		delete(list.collapsed, groupID)
	}
	list.rowsDirty = true
	list.ensureRows()
}

func (list *RoomList) OnKeyEvent(_ mauview.KeyEvent) bool {
	return false
}

func (list *RoomList) OnPasteEvent(_ mauview.PasteEvent) bool {
	return false
}

func (list *RoomList) OnMouseEvent(event mauview.MouseEvent) bool {
	if event.HasMotion() {
		return false
	}
	switch event.Buttons() {
	case tcell.WheelUp:
		list.addScrollOffset(-WheelScrollOffsetDiff)
		return true
	case tcell.WheelDown:
		list.addScrollOffset(WheelScrollOffsetDiff)
		return true
	case tcell.Button1:
		_, y := event.Position()
		list.lock.Lock()
		y += list.scrollOffset
		if y < 0 || y >= len(list.rows) {
			list.lock.Unlock()
			return false
		}
		row := list.rows[y]
		if row.group != nil {
			list.setCollapsed(row.group.Filter.ID(), !row.collapsed)
			list.lock.Unlock()
			return true
		}
		list.lock.Unlock()
		list.parent.SwitchRoom(row.room.RoomID)
		return true
	}
	return false
}

func (list *RoomList) addScrollOffset(offset int) {
	list.lock.Lock()
	defer list.lock.Unlock()
	list.scrollOffset += offset
	list.clampScrollOffset()
}

func (list *RoomList) Focus() {}
func (list *RoomList) Blur()  {}

func (list *RoomList) Draw(screen mauview.Screen) {
	list.lock.Lock()
	list.width, list.height = screen.Size()
	list.ensureRows()
	list.clampScrollOffset()
	rowSlice := list.rows[min(len(list.rows), list.scrollOffset):min(len(list.rows), list.scrollOffset+list.height)]
	selected := list.selected
	width := list.width
	list.lock.Unlock()

	for y, row := range rowSlice {
		if row.group != nil {
			list.drawGroupHeader(screen, y, width, row.group, row.collapsed)
		} else {
			list.drawRoom(screen, y, width, row.room, row.room.RoomID == selected)
		}
	}
}

func formatUnreadCount(count int, highlight bool) string {
	str := "99+"
	if count < 1000 {
		str = strconv.Itoa(count)
	}
	if highlight {
		str += "!"
	}
	return fmt.Sprintf("(%s)", str)
}

func (list *RoomList) drawRoom(screen mauview.Screen, y, width int, room *store.RoomListEntry, selected bool) {
	style := tcell.StyleDefault.
		Foreground(list.mainTextColor).
		Bold(room.MarkedUnread || room.UnreadNotifications > 0 || room.UnreadHighlights > 0)
	if selected {
		style = style.
			Foreground(list.selectedTextColor).
			Background(list.selectedBackgroundColor)
	}

	widget.WriteLinePadded(screen, mauview.AlignLeft, room.Name, 0, y, width, style)

	if room.UnreadMessages > 0 {
		unreadMessageCount := formatUnreadCount(room.UnreadMessages, room.UnreadHighlights > 0)
		widget.WriteLine(screen, mauview.AlignRight, unreadMessageCount, width-7, y, 7, style)
	}
}

func (list *RoomList) drawGroupHeader(screen mauview.Screen, y, width int, group *store.RoomListGroup, collapsed bool) {
	glyph := "▼"
	badge := ""
	if collapsed {
		glyph = "▶"
		if group.UnreadRooms > 0 {
			badge = formatUnreadCount(group.UnreadRooms, group.Highlights)
		}
	}
	style := tcell.StyleDefault.
		Foreground(list.mainTextColor).
		Bold(collapsed && group.Highlights)
	count := " " + strconv.Itoa(len(group.Rooms))
	// Truncate the name first, so the glyph and counts always fit.
	nameWidth := width - runewidth.StringWidth(glyph) - 1 - runewidth.StringWidth(count)
	if badge != "" {
		nameWidth -= runewidth.StringWidth(badge) + 1
	}
	name := runewidth.Truncate(group.Filter.Name(), max(nameWidth, 0), "…")
	widget.WriteLinePadded(screen, mauview.AlignLeft, glyph+" "+name+count, 0, y, width, style)
	if badge != "" {
		badgeWidth := runewidth.StringWidth(badge)
		widget.WriteLine(screen, mauview.AlignRight, badge, width-badgeWidth, y, badgeWidth, style)
	}
}
