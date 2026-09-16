package textui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Keys from a key map file must equal the keys tcell reports, or bindings silently do nothing.
func TestKeyMapMatchesTerminalKeys(t *testing.T) {
	cases := map[string]*tcell.EventKey{
		"w":       tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModNone),
		"W":       tcell.NewEventKey(tcell.KeyRune, 'W', tcell.ModShift),
		".":       tcell.NewEventKey(tcell.KeyRune, '.', tcell.ModNone),
		"Space":   tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone),
		"Tab":     tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
		"Backtab": tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift),
		"Enter":   tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
		"Esc":     tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		"F10":     tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone),
		"Up":      tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone),
		"Ctrl+x":  tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl),
	}
	for name, event := range cases {
		if fromFile, fromTerminal := UIKeyFromString(name), toUIKey(event); fromFile != fromTerminal {
			t.Errorf("%s: key map %v, terminal %v", name, fromFile, fromTerminal)
		}
	}
}
