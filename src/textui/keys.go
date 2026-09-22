package textui

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
)

// Commands are bound to keys in a key map file:
//
//	% main
//	w -> north
//	Ctrl+x -> examine
//	% targeting
//	Enter -> target_confirm
//
// and to functions with SetCommand.

func (u *UI) SetCommand(command string, action func()) {
	u.commandTable[command] = action
}

func (u *UI) appInputCapture(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyCtrlC {
		_, frontPanel := u.pages.GetFrontPanel()
		if inventory, isInventory := frontPanel.(*TextInventory); isInventory {
			inventory.handleInput(event)
			return nil // don't forward, or else we will quit
		}
		u.QuitGame()
		return nil
	}
	return event
}

func (u *UI) handleMainInput(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyCtrlC {
		return ev
	}
	u.mapOverlay.ClearAll()
	u.executePlayerCommand(u.getCommandForKey(toUIKey(ev)))
	return nil
}

// executePlayerCommand is how the game is driven by the player.
func (u *UI) executePlayerCommand(command string) {
	if command == "" {
		return
	}
	if action, ok := u.commandTable[command]; ok {
		action()
	} else {
		u.Print(fmt.Sprintf("Unknown command: %s", command))
	}
}

func (u *UI) directionalWrapperWithoutAlphabet(originalCapture func(event *tcell.EventKey) *tcell.EventKey) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && (event.Rune() >= 'a' && event.Rune() <= 'z') || (event.Rune() >= 'A' && event.Rune() <= 'Z') {
			if originalCapture != nil {
				return originalCapture(event)
			}
			return event
		}
		return u.directionalWrapper(originalCapture)(event)
	}
}

// directionalWrapper turns the movement commands into arrow keys, so widgets follow the key map.
func (u *UI) directionalWrapper(originalCapture func(event *tcell.EventKey) *tcell.EventKey) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		command := u.getCommandForKey(toUIKey(event))

		if command == "wait" {
			event = tcell.NewEventKey(tcell.KeyEscape, ' ', tcell.ModNone)
		}

		switch direction, possible := directionFromCommand(command); {
		case !possible:
		case direction == geometry.North:
			event = tcell.NewEventKey(tcell.KeyUp, ' ', tcell.ModNone)
		case direction == geometry.South:
			event = tcell.NewEventKey(tcell.KeyDown, ' ', tcell.ModNone)
		case direction == geometry.West:
			event = tcell.NewEventKey(tcell.KeyLeft, ' ', tcell.ModNone)
		case direction == geometry.East:
			event = tcell.NewEventKey(tcell.KeyRight, ' ', tcell.ModNone)
		}
		if originalCapture != nil {
			return originalCapture(event)
		}
		return event
	}
}

func (u *UI) GetKeysForCommandAsString(layer KeyLayer, command string) string {
	var keys []string
	for key, c := range u.keyTable[layer] {
		if c == command && key.name == " " {
			keys = append(keys, "Space")
		} else if c == command && key.name != "" {
			keys = append(keys, key.name)
		}
	}
	slices.SortStableFunc(keys, cmp.Compare[string])
	return strings.Join(keys, ", ")
}

func (u *UI) getCommandForKey(key UIKey) string {
	return u.keyTable[KeyLayerMain][key]
}

func (u *UI) getTargetingCommandForKey(key UIKey) string {
	return u.keyTable[KeyLayerTargeting][key]
}

func directionFromCommand(command string) (geometry.CompassDirection, bool) {
	switch strings.TrimPrefix(command, "force_") {
	case "north":
		return geometry.North, true
	case "south":
		return geometry.South, true
	case "west":
		return geometry.West, true
	case "east":
		return geometry.East, true
	case "northwest":
		return geometry.NorthWest, true
	case "northeast":
		return geometry.NorthEast, true
	case "southwest":
		return geometry.SouthWest, true
	case "southeast":
		return geometry.SouthEast, true
	}
	return geometry.East, false
}

// ShowKeyBindings lists the main layer's commands in two columns, named by friendlyNames.
func (u *UI) ShowKeyBindings(friendlyNames map[string]string, leftColCommands, rightColCommands []string) {
	rowsOfTable := make([]fxtools.TableRow, 0)
	lines := max(len(leftColCommands), len(rightColCommands))
	for i := 0; i < lines; i++ {
		leftKeys, leftCmd, rightKeys, rightCmd := "", "", "", ""
		if i < len(leftColCommands) {
			leftCmd = leftColCommands[i]
			leftKeys = u.GetKeysForCommandAsString(KeyLayerMain, leftCmd)
		}
		if i < len(rightColCommands) {
			rightCmd = rightColCommands[i]
			rightKeys = u.GetKeysForCommandAsString(KeyLayerMain, rightCmd)
		}
		rowsOfTable = append(rowsOfTable, fxtools.TableRow{
			Columns: []string{" ", escapeKeys(leftKeys), friendlyNames[leftCmd], "   ", escapeKeys(rightKeys), friendlyNames[rightCmd]},
		})
	}
	commandTableRender := fxtools.TableLayout(rowsOfTable, []fxtools.TextAlignment{
		fxtools.AlignLeft, fxtools.AlignLeft, fxtools.AlignLeft, fxtools.AlignLeft, fxtools.AlignLeft, fxtools.AlignLeft,
	})
	u.OpenTextWindow(strings.Join(commandTableRender, "\n"))
}

func escapeKeys(keys string) string {
	return cview.Escape(keys)
}

func toUIKey(keyEvent *tcell.EventKey) UIKey {
	name := tcell.KeyNames[keyEvent.Key()]
	ch := rune(0)
	if keyEvent.Key() == tcell.KeyRune {
		name = string(keyEvent.Rune())
		ch = keyEvent.Rune()
	}
	mod := keyEvent.Modifiers()
	if k := keyEvent.Key(); k == tcell.KeyRune || k == tcell.KeyBacktab || k >= tcell.KeyCtrlSpace && k <= tcell.KeyCtrlUnderscore {
		mod = tcell.ModNone // the rune or key already says it
	}
	return UIKey{
		mod:  mod,
		key:  keyEvent.Key(),
		ch:   ch,
		name: name,
	}
}

type KeyLayer int

const (
	KeyLayerMain KeyLayer = iota
	KeyLayerTargeting
)

func (u *UI) loadKeyMap(file io.Reader) {
	u.keyTable = map[KeyLayer]map[UIKey]string{
		KeyLayerMain:      make(map[UIKey]string),
		KeyLayerTargeting: make(map[UIKey]string),
	}

	scanner := bufio.NewScanner(file)
	currentLayer := KeyLayerMain
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "%") {
			switch strings.TrimSpace(strings.TrimPrefix(line, "%")) {
			case "main":
				currentLayer = KeyLayerMain
			case "targeting":
				currentLayer = KeyLayerTargeting
			}
			continue
		}
		parts := strings.Split(line, "->")
		if len(parts) == 2 {
			key := UIKeyFromString(strings.TrimSpace(parts[0]))
			u.keyTable[currentLayer][key] = strings.TrimSpace(parts[1])
		}
	}
}

type UIKey struct {
	mod  tcell.ModMask
	key  tcell.Key
	ch   rune
	name string
}

var (
	ctrlComboRegex  = regexp.MustCompile(`^Ctrl\+(\w+)$`)
	shiftComboRegex = regexp.MustCompile(`^Shift\+(\w+)$`)
)

func UIKeyFromString(s string) UIKey {
	if printableRune, ok := ParsePrintableKey(s); ok {
		return Letter(printableRune)
	}
	keyString := s
	var mods tcell.ModMask
	// not a single key, so it's either a combo or a function key or a non-printable key
	if matches := ctrlComboRegex.FindStringSubmatch(s); matches != nil {
		keyString = matches[1]
		mods = tcell.ModCtrl
	}
	if matches := shiftComboRegex.FindStringSubmatch(s); matches != nil {
		keyString = matches[1]
		mods = tcell.ModShift
	}
	if mods == tcell.ModCtrl {
		if printableRune, ok := ParsePrintableKey(keyString); ok && printableRune >= 'a' && printableRune <= 'z' {
			key := tcell.KeyCtrlA + tcell.Key(printableRune-'a')
			return UIKey{key: key, name: tcell.KeyNames[key]}
		}
	}
	if tcellKey, ok := ParseNonPrintableKey(keyString); ok {
		return UIKey{key: tcellKey, name: tcell.KeyNames[tcellKey], mod: mods}
	}
	return UIKey{name: "unknown"}
}

func ParsePrintableKey(s string) (rune, bool) {
	if strings.TrimSpace(strings.ToLower(s)) == "space" {
		s = " "
	}
	if runes := []rune(s); len(runes) == 1 {
		return runes[0], true
	}
	return 0, false
}

var nonPrintable = map[string]tcell.Key{
	"tab":       tcell.KeyTab,
	"backtab":   tcell.KeyBacktab,
	"enter":     tcell.KeyEnter,
	"esc":       tcell.KeyEscape,
	"escape":    tcell.KeyEscape,
	"del":       tcell.KeyDelete,
	"backspace": tcell.KeyBackspace2,
	"up":        tcell.KeyUp,
	"down":      tcell.KeyDown,
	"left":      tcell.KeyLeft,
	"right":     tcell.KeyRight,
	"home":      tcell.KeyHome,
	"end":       tcell.KeyEnd,
	"pgup":      tcell.KeyPgUp,
	"pgdn":      tcell.KeyPgDn,
	"f1":        tcell.KeyF1,
	"f2":        tcell.KeyF2,
	"f3":        tcell.KeyF3,
	"f4":        tcell.KeyF4,
	"f5":        tcell.KeyF5,
	"f6":        tcell.KeyF6,
	"f7":        tcell.KeyF7,
	"f8":        tcell.KeyF8,
	"f9":        tcell.KeyF9,
	"f10":       tcell.KeyF10,
	"f11":       tcell.KeyF11,
	"f12":       tcell.KeyF12,
}

func ParseNonPrintableKey(s string) (tcell.Key, bool) {
	key, ok := nonPrintable[strings.TrimSpace(strings.ToLower(s))]
	return key, ok
}

func Letter(letter rune) UIKey {
	return UIKey{ch: letter, name: string(letter), key: tcell.KeyRune}
}

func (k UIKey) String() string {
	return fmt.Sprintf("UIKey{mod: %d, key: %d, ch: %d, name: %s}", k.mod, k.key, k.ch, k.name)
}
