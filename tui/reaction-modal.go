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
	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"go.mau.fi/util/variationselector"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/tui/config"
)

// DefaultQuickReactions are shown in the reaction picker unless overridden with the quick_reactions preference.
var DefaultQuickReactions = []string{"👍", "❤️", "😂", "😮", "😢", "🎉", "✅", "👀"}

const (
	reactionCellWidth = 4
	// Border + padding on each side.
	reactionModalPadding = 2
)

// ReactionModal is a small popup for reacting to a message with one of a few quick reactions.
// Picking a reaction that the user has already sent removes it instead.
type ReactionModal struct {
	parent   *RoomView
	target   *database.Event
	emojis   []string
	selected int

	// Area of the modal from the last draw, used for mouse hit testing.
	x, y, width, height int
}

func NewReactionModal(parent *RoomView, target *database.Event) *ReactionModal {
	emojis := parent.config.Preferences.QuickReactions
	if len(emojis) == 0 {
		emojis = DefaultQuickReactions
	}
	normalized := make([]string, len(emojis))
	for i, emoji := range emojis {
		normalized[i] = variationselector.Add(emoji)
	}
	return &ReactionModal{
		parent: parent,
		target: target,
		emojis: normalized,
	}
}

// ownReactionIDs returns the IDs of the user's existing reactions with the given emoji. Reactions sent by other
// clients may or may not include the variation selector, so both forms are checked.
func (rm *ReactionModal) ownReactionIDs(emoji string) []id.EventID {
	if ids := rm.target.OwnReactions[emoji]; len(ids) > 0 {
		return ids
	}
	return rm.target.OwnReactions[variationselector.Remove(emoji)]
}

// hasOwnReaction returns true if the user has already reacted with the given emoji.
func (rm *ReactionModal) hasOwnReaction(emoji string) bool {
	return len(rm.ownReactionIDs(emoji)) > 0
}

func (rm *ReactionModal) pick(index int) {
	rm.parent.parent.HideModal()
	if index < 0 || index >= len(rm.emojis) {
		return
	}
	emoji := rm.emojis[index]
	if ownIDs := rm.ownReactionIDs(emoji); len(ownIDs) > 0 {
		go rm.parent.Redact(ownIDs[0], "")
	} else {
		go rm.parent.SendReaction(rm.target.ID, emoji)
	}
}

func (rm *ReactionModal) Draw(screen mauview.Screen) {
	screenWidth, screenHeight := screen.Size()
	rm.width = len(rm.emojis)*reactionCellWidth + reactionModalPadding
	rm.height = 3
	rm.x = max((screenWidth-rm.width)/2, 0)
	rm.y = max((screenHeight-rm.height)/2, 0)
	box := mauview.NewProxyScreen(screen, rm.x, rm.y, rm.width, rm.height)

	border := tcell.StyleDefault.Foreground(mauview.Styles.BorderColor)
	box.Fill(' ', tcell.StyleDefault)
	for x := 1; x < rm.width-1; x++ {
		box.SetContent(x, 0, tcell.RuneHLine, nil, border)
		box.SetContent(x, 2, tcell.RuneHLine, nil, border)
	}
	box.SetContent(0, 0, tcell.RuneULCorner, nil, border)
	box.SetContent(rm.width-1, 0, tcell.RuneURCorner, nil, border)
	box.SetContent(0, 2, tcell.RuneLLCorner, nil, border)
	box.SetContent(rm.width-1, 2, tcell.RuneLRCorner, nil, border)
	box.SetContent(0, 1, tcell.RuneVLine, nil, border)
	box.SetContent(rm.width-1, 1, tcell.RuneVLine, nil, border)
	mauview.PrintWithStyle(box, " React ", 2, 0, rm.width-4, mauview.AlignLeft, border)

	for i, emoji := range rm.emojis {
		style := tcell.StyleDefault
		if rm.hasOwnReaction(emoji) {
			style = style.Background(tcell.ColorDarkGreen)
		}
		if i == rm.selected {
			style = style.Reverse(true)
		}
		cellX := 1 + i*reactionCellWidth
		for x := range reactionCellWidth {
			box.SetContent(cellX+x, 1, ' ', nil, style)
		}
		// Variation selectors are dropped for display, because terminals and tcell often disagree on the width of
		// emoji presentation sequences, which corrupts the rest of the line.
		mauview.PrintWithStyle(box, variationselector.Remove(emoji), cellX+1, 1, reactionCellWidth-1, mauview.AlignLeft, style)
	}
}

func (rm *ReactionModal) OnKeyEvent(event mauview.KeyEvent) bool {
	kb := config.Keybind{
		Key: event.Key(),
		Ch:  event.Rune(),
		Mod: event.Modifiers(),
	}
	switch rm.parent.config.Keybindings.Modal[kb] {
	case "cancel":
		rm.parent.parent.HideModal()
		return true
	case "confirm":
		rm.pick(rm.selected)
		return true
	case "select_next":
		rm.selected = (rm.selected + 1) % len(rm.emojis)
		return true
	case "select_prev":
		rm.selected = (rm.selected - 1 + len(rm.emojis)) % len(rm.emojis)
		return true
	}
	switch event.Key() {
	case tcell.KeyRight:
		rm.selected = (rm.selected + 1) % len(rm.emojis)
	case tcell.KeyLeft:
		rm.selected = (rm.selected - 1 + len(rm.emojis)) % len(rm.emojis)
	case tcell.KeyRune:
		// Number keys pick a reaction directly.
		if ch := event.Rune(); ch >= '1' && ch <= '9' {
			rm.pick(int(ch - '1'))
		} else if ch == 'q' {
			rm.parent.parent.HideModal()
		}
	default:
		return false
	}
	return true
}

func (rm *ReactionModal) OnMouseEvent(event mauview.MouseEvent) bool {
	if event.HasMotion() {
		return false
	}
	switch event.Buttons() {
	case tcell.Button1, tcell.Button2:
		x, y := event.Position()
		if x < rm.x || x >= rm.x+rm.width || y < rm.y || y >= rm.y+rm.height {
			// Clicking outside the modal closes it.
			rm.parent.parent.HideModal()
			return true
		}
		if y == rm.y+1 && x > rm.x && x < rm.x+rm.width-1 {
			rm.pick((x - rm.x - 1) / reactionCellWidth)
		}
		return true
	}
	return false
}

func (rm *ReactionModal) OnPasteEvent(_ mauview.PasteEvent) bool {
	return false
}

func (rm *ReactionModal) Focus() {}
func (rm *ReactionModal) Blur()  {}
