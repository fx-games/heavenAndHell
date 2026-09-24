package gui

import (
	"fmt"
	"image/color"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"

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
	labelW      [2]int // per column: the long trait values on the right must not widen the left
	colW        [2]int
	plusRects   []geometry.Rect // one per attribute row, empty where the row has no "+"
	focus       int             // which attribute the keyboard is on
}

const (
	sheetPad      = 12 // inside the border, around the columns
	sheetColGap   = 20
	sheetPlusSize = 18
)

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
	if _, open := u.GetTopModal().(*CharacterSheet); open {
		u.PopWidget()
		return
	}
	s := &CharacterSheet{u: u}
	s.Layout()
	u.PushWidget(s)
}

// columns splits the sections into the left and right column: the attributes sit under the left one.
func (s *CharacterSheet) columns() (left, right []common.SheetSection) {
	state := s.u.context.GameState()
	sections := state.GetPlayer().SheetSections()
	if state.NewGamePlus() { // on a first run it only ever says "Normal", which is not worth a heading
		sections = append(sections, state.ThreatSection())
	}
	for _, sec := range sections {
		switch sec.Title {
		case "Condition": // Life and Mana read fine without a heading; a blank title leaves just a gap
			sec.Title = ""
			left = append(left, sec)
		case "Character":
			left = append(left, sec)
		default:
			right = append(right, sec)
		}
	}
	return left, right
}

func (s *CharacterSheet) attributes() []common.SheetAttribute {
	state := s.u.context.GameState()
	return append(state.GetPlayer().SheetAttributes(), state.SkillRows(state.GetPlayer())...)
}

func (s *CharacterSheet) Layout() {
	r := s.u.GetRenderer()
	_, h := r.MeasureString("M")
	s.lineH = int(h) + 3
	width := func(text string) int { w, _ := r.MeasureString(text); return int(w) }
	left, right := s.columns()
	attrs := s.attributes()
	s.labelW, s.colW = [2]int{}, [2]int{}
	var valueW, height [2]int
	// the attributes hang under the left column, headed only by a gap unless points are waiting
	height[0] = len(attrs)*s.lineH + s.titleH(s.pointsTitle())
	s.colW[0] = width("99 point(s) to spend")
	for _, at := range attrs {
		s.labelW[0] = max(s.labelW[0], width(at.Label)+10)
		valueW[0] = max(valueW[0], width(s.attrValue(at))+sheetPlusSize+6) // the "+" sits at the right edge
	}
	for i, sections := range [2][]common.SheetSection{left, right} {
		height[i] += drawSectionBlock(nil, sections, 0, 0, 0, s.lineH)
		for _, sec := range sections {
			if len(sec.Rows) == 0 && len(sec.Notes) == 0 {
				continue
			}
			s.colW[i] = max(s.colW[i], width(sec.Title))
			for _, row := range sec.Rows {
				s.labelW[i] = max(s.labelW[i], width(row[0])+10)
				valueW[i] = max(valueW[i], width(row[1]))
			}
			for _, note := range sec.Notes {
				s.colW[i] = max(s.colW[i], width(note))
			}
		}
		s.colW[i] = max(s.colW[i], s.labelW[i]+valueW[i])
	}
	s.size = geometry.Point{X: s.colW[0] + sheetColGap + s.colW[1], Y: max(height[0], height[1]) + s.lineH/2}
	// centre over the map, not the screen: the sidebar stays uncovered so the inventory can be used alongside
	area := s.u.GetScreenSize().Sub(geometry.Point{X: s.u.GetUIBarWidth()})
	s.topLeft = area.Sub(s.size).Div(2)
	s.topLeft.X = max(s.topLeft.X, sheetPad)
	s.topLeft.X, s.topLeft.Y = max(s.topLeft.X, sheetPad), max(s.topLeft.Y, sheetPad)
	if s.focus >= len(attrs) {
		s.focus = 0
	}
}

// titleH is a heading's line, or the half-line gap drawSectionBlock leaves for an untitled section.
func (s *CharacterSheet) titleH(title string) int {
	if title == "" {
		return s.lineH / 2
	}
	return s.lineH
}

// pointsTitle heads the attributes only while there are stat points to spend.
func (s *CharacterSheet) pointsTitle() string {
	if h := s.u.context.GameState().GetPlayer().Hero(); h != nil && h.SkillPts > 0 {
		return fmt.Sprintf("%d stat, %d skill point(s)", h.StatPts, h.SkillPts)
	} else if h != nil && h.StatPts > 0 {
		return fmt.Sprintf("%d point(s) to spend", h.StatPts)
	}
	return ""
}

func (s *CharacterSheet) attrValue(at common.SheetAttribute) string {
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
	y := s.drawSections(left, s.topLeft.X, s.topLeft.Y, 0)
	s.drawSections(right, s.topLeft.X+s.colW[0]+sheetColGap, s.topLeft.Y, 1)
	s.drawAttributes(s.topLeft.X, y)
}

// drawSections draws one column and returns the y it stopped at.
func (s *CharacterSheet) drawSections(sections []common.SheetSection, x, y, col int) int {
	return drawSectionBlock(s.u.GetRenderer(), sections, x, y, s.labelW[col], s.lineH)
}

func (s *CharacterSheet) drawAttributes(x, y int) {
	r := s.u.GetRenderer()
	attrs := s.attributes()
	title := s.pointsTitle()
	y += s.titleH(title)
	r.DrawTTFOnScreen(float64(x), float64(y), title, defs.PositiveHighlightColor)
	s.plusRects = make([]geometry.Rect, len(attrs))
	for i, at := range attrs {
		y += s.lineH
		valueColor := defs.LightTextColor
		if at.Base >= at.Max {
			valueColor = defs.DarkGray // capped for this class: no point ever goes in here again
		}
		r.DrawTTFOnScreen(float64(x), float64(y), at.Label, defs.DarkTextColor)
		r.DrawTTFOnScreen(float64(x+s.labelW[0]), float64(y), s.attrValue(at), valueColor)
		if !at.CanRaise {
			continue
		}
		plus := geometry.NewRect(x+s.colW[0]-sheetPlusSize, y-s.lineH+6, x+s.colW[0], y+6)
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
	s.u.RefreshIconBars() // a skill raised from 0 joins the skill bar now, not next turn
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
	if s.overSidebar(x) { // let the inventory take it: equipping with the sheet open shows the new numbers live
		return false
	}
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

func (s *CharacterSheet) overSidebar(x int) bool {
	return x >= s.u.GetScreenSize().X-s.u.GetUIBarWidth()
}

func (s *CharacterSheet) OnMouseWheel(x int, y int, dy float64) bool { return true }
func (s *CharacterSheet) ShouldClose() bool                          { return s.shouldClose }
func (s *CharacterSheet) IsNull() bool                               { return false }
