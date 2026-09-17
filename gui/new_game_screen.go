package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
)

const newGameIconSize = 32 // 8px tiles at scale 4

// NewGameScreen picks class (rows) and gender (columns), permadeath and a name on one screen.
// Rows 0..len(classes)-1 are the icon grid, then the permadeath checkbox, the name field, the start and back buttons.
type NewGameScreen struct {
	u           *UIController
	classes     []defs.LoadoutDefinition
	start       func(class defs.LoadoutDefinition, permadeath bool, name string)
	back        func()
	name        *TextInput
	rolledName  string // last random name we put into the field; anything else was typed by the player
	class, col  int
	focus       int
	permadeath  bool
	shouldClose bool
	topLeft     geometry.Point
	size        geometry.Point
	labelW      int
	lineH       int
}

func (u *UIController) OpenNewGameScreen(start func(class defs.LoadoutDefinition, permadeath bool, name string), back func()) {
	s := &NewGameScreen{u: u, classes: common.PlayableClasses(u.configuration, u.context.Definitions.ClassDefinitions), start: start, back: back}
	s.name = NewTextInput(u, geometry.Point{}, 16, 0, 1, func(EndAction, string) {})
	s.Layout()
	u.PushWidget(s)
}

func (s *NewGameScreen) permadeathRow() int { return len(s.classes) }
func (s *NewGameScreen) nameRow() int       { return len(s.classes) + 1 }
func (s *NewGameScreen) startRow() int      { return len(s.classes) + 2 }
func (s *NewGameScreen) backRow() int       { return len(s.classes) + 3 }

func (s *NewGameScreen) Layout() {
	r := s.u.GetRenderer()
	w, h := r.MeasureString("Name: MMMMMMMMMMMMMMMM_")
	s.lineH = int(h) + 12
	for _, c := range s.classes {
		lw, _ := r.MeasureString(c.Name)
		s.labelW = max(s.labelW, int(lw)+16)
	}
	s.size = geometry.Point{X: max(int(w), s.labelW+2*newGameIconSize+8), Y: len(s.classes)*(newGameIconSize+8) + 4*s.lineH}
	screen := s.u.GetScreenSize()
	s.topLeft = screen.Sub(s.size).Div(2)
}

// rowRect is the clickable area of a row; for grid rows col picks the icon cell.
func (s *NewGameScreen) rowRect(row, col int) geometry.Rect {
	x, y := s.topLeft.X, s.topLeft.Y
	if row < len(s.classes) {
		x += s.labelW + col*(newGameIconSize+8)
		y += row * (newGameIconSize + 8)
		return geometry.NewRect(x, y, x+newGameIconSize, y+newGameIconSize)
	}
	y += len(s.classes)*(newGameIconSize+8) + (row-len(s.classes))*s.lineH
	return geometry.NewRect(x, y, x+s.size.X, y+s.lineH)
}

func (s *NewGameScreen) icon(row, col int) int32 {
	if col == 1 {
		return defs.FemaleIconByClass[s.classes[row].InternalName]
	}
	return s.classes[row].Icon
}

// rollName puts a random name for the selected gender into the name field unless the player typed one.
func (s *NewGameScreen) rollName() {
	if t := s.name.currentText; t != "" && t != s.rolledName {
		return
	}
	s.rolledName = common.RandomHeroName(s.col == 1)
	s.name.currentText = s.rolledName
}

func (s *NewGameScreen) Draw() {
	r := s.u.GetRenderer()
	pad := geometry.Point{X: 40, Y: 40}
	r.DrawDefaultBorder(s.topLeft.Sub(pad.Div(2)), s.size.Add(pad))
	colorOf := func(highlighted bool) color.Color {
		if highlighted {
			return defs.LightTextColor
		}
		return defs.DarkTextColor
	}
	_, h := r.MeasureString("M")
	for row, c := range s.classes {
		y := s.rowRect(row, 0).Min.Y
		r.DrawTTFOnScreen(float64(s.topLeft.X), float64(y+(newGameIconSize+int(h))/2), c.Name, colorOf(row == s.class))
		for col := 0; col < 2; col++ {
			cell := s.rowRect(row, col).Min
			r.DrawOnScreenWithScale(cell.X, cell.Y, s.icon(row, col), 4)
			if row == s.class && col == s.col {
				r.DrawOnScreenWithScale(cell.X, cell.Y, defs.IconTargeting, 4)
			}
		}
	}
	check := "[ ] Permadeath"
	if s.permadeath {
		check = "[x] Permadeath"
	}
	nameText := "Name: " + s.name.currentText
	if s.focus == s.nameRow() {
		nameText += "_"
	}
	for i, text := range []string{check, nameText, "Start game", "Back"} {
		row := s.permadeathRow() + i
		y := s.rowRect(row, 0).Min.Y + (s.lineH+int(h))/2
		r.DrawTTFOnScreen(float64(s.topLeft.X), float64(y), text, colorOf(row == s.focus))
	}
}

func (s *NewGameScreen) OnCommand(command CommandType) bool {
	switch command {
	case PlayerCommandUp:
		s.focus = max(s.focus-1, 0)
	case PlayerCommandDown:
		s.focus = min(s.focus+1, s.backRow())
	case PlayerCommandLeft, PlayerCommandRight:
		if s.focus < len(s.classes) {
			s.col = 1 - s.col
			s.rollName()
		}
	case PlayerCommandConfirm:
		s.activate(s.focus)
		return true
	case PlayerCommandCancel:
		s.activate(s.backRow())
		return true
	}
	if s.focus < len(s.classes) {
		s.class = s.focus
	}
	return true
}

// activate toggles the checkbox, goes back to the menu that opened the screen, or starts the game
// (Enter on a class or the name field starts too).
func (s *NewGameScreen) activate(row int) {
	switch row {
	case s.permadeathRow():
		s.permadeath = !s.permadeath
		return
	case s.backRow():
		s.shouldClose = true
		s.back()
		return
	}
	class := s.classes[s.class]
	class.Icon = s.icon(s.class, s.col)
	s.shouldClose = true
	s.start(class, s.permadeath, s.name.currentText)
}

func (s *NewGameScreen) OnKeyPressed(key ebiten.Key, shiftPressed bool) {
	if s.focus == s.nameRow() {
		s.name.OnKeyPressed(key, shiftPressed)
		return
	}
	switch key {
	case ebiten.KeyW:
		s.OnCommand(PlayerCommandUp)
	case ebiten.KeyS:
		s.OnCommand(PlayerCommandDown)
	case ebiten.KeyA:
		s.OnCommand(PlayerCommandLeft)
	case ebiten.KeyD:
		s.OnCommand(PlayerCommandRight)
	case ebiten.KeySpace:
		if s.focus == s.permadeathRow() {
			s.activate(s.focus)
		}
	}
}

func (s *NewGameScreen) OnMouseClicked(button ebiten.MouseButton, x int, y int) bool {
	p := geometry.Point{X: x, Y: y}
	for row := 0; row <= s.backRow(); row++ {
		for col := 0; col < 2; col++ {
			if !s.rowRect(row, col).Contains(p) {
				continue
			}
			s.focus = row
			if row < len(s.classes) {
				s.class, s.col = row, col
				s.rollName()
			} else if row != s.nameRow() {
				s.activate(row)
			}
			return true
		}
	}
	return true
}

func (s *NewGameScreen) OnMouseMoved(x int, y int) (bool, Tooltip)  { return true, NoTooltip{} }
func (s *NewGameScreen) OnMouseWheel(x int, y int, dy float64) bool { return true }
func (s *NewGameScreen) ShouldClose() bool                          { return s.shouldClose }
