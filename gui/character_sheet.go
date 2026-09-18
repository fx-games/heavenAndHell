package gui

import (
	"fmt"
	"image/color"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"

	"github.com/hajimehoshi/ebiten/v2"
)

// CharacterSheet is the 'c' window and the level-up screen in one: two columns of titled sections, with a "+"
// next to every attribute a stat point can still go into. It reads the player fresh on every draw, so spending a
// point updates the numbers around it right away.
type CharacterSheet struct {
	u           *UIController
	shouldClose bool
	topLeft     geometry.Point
	size        geometry.Point
	lineH       int
	labelW      int
	valueW      int
	colW        int
	plusRects   []geometry.Rect // one per attribute row, empty where the row has no "+"
	focus       int             // which attribute the keyboard is on
}

const (
	sheetPad      = 12 // inside the border, around the columns
	sheetColGap   = 20
	sheetPlusSize = 18
)

// sheetHeaderColors give each section its own hue, so the eye finds a block without reading the titles.
var sheetHeaderColors = map[string]color.Color{
	"Character":   defs.Brown,
	"Condition":   defs.InvColorAmulet,
	"Attributes":  defs.Brown,
	"Combat":      defs.InvColorMeleeWeapon,
	"Resistances": defs.InvColorUnique,
	"Traits":      defs.InvColorPotion,
	"World":       defs.Red,
	"Creature":    defs.Brown,
	"Behavior":    defs.InvColorPotion,
	"Equipment":   defs.InvColorMeleeWeapon,
	"Afflictions": defs.Red,
	"Matchup":     defs.InvColorUnique,
}

// sheetValueColors colour the values that mean something on sight; everything else stays the default gold.
var sheetValueColors = map[string]color.Color{
	"Life":      defs.Red,
	"Mana":      defs.Blue,
	"Gold":      defs.Yellow,
	"Fire":      defs.Red,
	"Lightning": defs.Yellow,
	"Magic":     defs.MagicHighlightColor,
	"Threat":    defs.Red,
}

func sheetColor(m map[string]color.Color, key string, fallback color.Color) color.Color {
	if c, ok := m[key]; ok {
		return c
	}
	return fallback
}

// OpenCharacterSheet puts the character sheet up, or takes it down if it is already up.
func (u *UIController) OpenCharacterSheet() {
	if _, open := u.modal.(*CharacterSheet); open {
		u.PopWidget()
		return
	}
	s := &CharacterSheet{u: u}
	s.Layout()
	u.PushWidget(s)
}

// columns splits the sections into the left and right column: the attributes sit under the left one.
func (s *CharacterSheet) columns() (left, right []common.D1SheetSection) {
	state := s.u.context.GameState()
	sections := state.GetPlayer().D1SheetSections()
	if state.D1NewGamePlus() { // on a first run it only ever says "Normal", which is not worth a heading
		sections = append(sections, state.D1ThreatSection())
	}
	for _, sec := range sections {
		switch sec.Title {
		case "Character", "Condition":
			left = append(left, sec)
		default:
			right = append(right, sec)
		}
	}
	return left, right
}

func (s *CharacterSheet) attributes() []common.D1SheetAttribute {
	return s.u.context.GameState().GetPlayer().D1SheetAttributes()
}

func (s *CharacterSheet) Layout() {
	r := s.u.GetRenderer()
	_, h := r.MeasureString("M")
	s.lineH = int(h) + 3
	s.labelW, s.valueW = 0, 0
	widest := func(dst *int, text string, gutter int) {
		w, _ := r.MeasureString(text)
		*dst = max(*dst, int(w)+gutter)
	}
	left, right := s.columns()
	// a section costs its rows plus a title line and a half-line gap; the attributes hang under the left column
	sideLines := func(sections []common.D1SheetSection) int {
		lines := 0
		for _, sec := range sections {
			lines += len(sec.Rows) + len(sec.Notes) + 2
			for _, row := range sec.Rows {
				widest(&s.labelW, row[0], 10)
				widest(&s.valueW, row[1], sheetPlusSize+6) // the "+" sits at the right edge of the value column
			}
		}
		return lines
	}
	attrs := s.attributes()
	for _, at := range attrs {
		widest(&s.labelW, at.Label, 10)
		widest(&s.valueW, s.attrValue(at), sheetPlusSize+6)
	}
	lines := max(sideLines(left)+len(attrs)+2, sideLines(right))
	s.colW = s.labelW + s.valueW
	s.size = geometry.Point{X: 2*s.colW + sheetColGap, Y: (lines + 1) * s.lineH}
	screen := s.u.GetScreenSize()
	s.topLeft = screen.Sub(s.size).Div(2)
	if s.focus >= len(attrs) {
		s.focus = 0
	}
}

func (s *CharacterSheet) attrValue(at common.D1SheetAttribute) string {
	if at.Total != at.Base {
		return fmt.Sprintf("%d (%d) / %d", at.Base, at.Total, at.Max)
	}
	return fmt.Sprintf("%d / %d", at.Base, at.Max)
}

func (s *CharacterSheet) Draw() {
	r := s.u.GetRenderer()
	pad := geometry.Point{X: 2 * sheetPad, Y: 2 * sheetPad}
	r.DrawColoredRect(s.topLeft.Sub(pad.Div(2)), s.size.Add(pad), color.RGBA{R: 12, G: 9, B: 7, A: 240})
	r.DrawDefaultBorder(s.topLeft.Sub(pad.Div(2)), s.size.Add(pad))

	left, right := s.columns()
	y := s.drawSections(left, s.topLeft.X, s.topLeft.Y)
	s.drawSections(right, s.topLeft.X+s.colW+sheetColGap, s.topLeft.Y)
	s.drawAttributes(s.topLeft.X, y)
}

// drawSections draws one column and returns the y it stopped at.
func (s *CharacterSheet) drawSections(sections []common.D1SheetSection, x, y int) int {
	return drawSectionBlock(s.u.GetRenderer(), sections, x, y, s.labelW, s.lineH)
}

func (s *CharacterSheet) drawAttributes(x, y int) {
	r := s.u.GetRenderer()
	attrs := s.attributes()
	points := 0
	if h := s.u.context.GameState().GetPlayer().Hero(); h != nil {
		points = h.StatPts
	}
	title := "Attributes"
	titleColor := sheetHeaderColors["Attributes"]
	if points > 0 {
		title = fmt.Sprintf("Attributes - %d point(s) to spend", points)
		titleColor = defs.PositiveHighlightColor
	}
	y += s.lineH
	r.DrawTTFOnScreen(float64(x), float64(y), title, titleColor)
	s.plusRects = make([]geometry.Rect, len(attrs))
	for i, at := range attrs {
		y += s.lineH
		valueColor := defs.LightTextColor
		if at.Base >= at.Max {
			valueColor = defs.DarkGray // capped for this class: no point ever goes in here again
		}
		r.DrawTTFOnScreen(float64(x), float64(y), at.Label, defs.DarkTextColor)
		r.DrawTTFOnScreen(float64(x+s.labelW), float64(y), s.attrValue(at), valueColor)
		if !at.CanRaise {
			continue
		}
		plus := geometry.NewRect(x+s.colW-sheetPlusSize, y-s.lineH+6, x+s.colW, y+6)
		s.plusRects[i] = plus
		fill := color.RGBA{R: 40, G: 30, B: 12, A: 255}
		if i == s.focus {
			fill = color.RGBA{R: 90, G: 66, B: 20, A: 255}
		}
		r.DrawColoredRect(plus.Min, plus.Size(), fill)
		r.DrawDefaultBorder(plus.Min, plus.Size())
		r.DrawTTFOnScreen(float64(plus.Min.X+5), float64(y), "+", defs.PositiveHighlightColor)
	}
}

// raise spends a point on attribute i and relays out, since the value widths can grow.
func (s *CharacterSheet) raise(i int) {
	attrs := s.attributes()
	if i < 0 || i >= len(attrs) || !attrs[i].CanRaise {
		return
	}
	attrs[i].Raise()
	s.Layout()
}

func (s *CharacterSheet) OnCommand(command CommandType) bool {
	attrs := s.attributes()
	switch command {
	case PlayerCommandUp:
		s.focus = max(s.focus-1, 0)
	case PlayerCommandDown:
		s.focus = min(s.focus+1, max(len(attrs)-1, 0))
	case PlayerCommandConfirm: // deliberately not Left/Right: WASD maps to those, and 'd' must not spend a point
		s.raise(s.focus)
	case PlayerCommandCancel:
		s.shouldClose = true
	}
	return true
}

func (s *CharacterSheet) OnMouseClicked(button ebiten.MouseButton, x int, y int) bool {
	p := geometry.Point{X: x, Y: y}
	for i, rect := range s.plusRects {
		if rect.Size().X > 0 && rect.Contains(p) {
			s.focus = i
			s.raise(i)
			return true
		}
	}
	if button == ebiten.MouseButtonRight {
		s.shouldClose = true
	}
	return true
}

func (s *CharacterSheet) OnMouseMoved(x int, y int) (bool, Tooltip) {
	p := geometry.Point{X: x, Y: y}
	for i, rect := range s.plusRects {
		if rect.Size().X > 0 && rect.Contains(p) {
			s.focus = i
		}
	}
	return true, NoTooltip{}
}

func (s *CharacterSheet) OnMouseWheel(x int, y int, dy float64) bool { return true }
func (s *CharacterSheet) ShouldClose() bool                          { return s.shouldClose }
func (s *CharacterSheet) IsNull() bool                               { return false }
