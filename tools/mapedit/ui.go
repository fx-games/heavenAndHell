package main

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// A tiny immediate-mode toolkit: enough for panels, buttons, lists and editable fields, and no more. Every
// widget draws itself and answers what the mouse did to it in the same call.

var face = text.NewGoXFace(basicfont.Face7x13)

const (
	lineH   = 14 // one row of text
	charW   = 7
	padding = 4
)

var (
	colBG        = color.RGBA{24, 24, 28, 255}
	colPanel     = color.RGBA{36, 36, 42, 255}
	colPanelAlt  = color.RGBA{30, 30, 35, 255}
	colLine      = color.RGBA{58, 58, 68, 255}
	colText      = color.RGBA{214, 214, 222, 255}
	colTextDim   = color.RGBA{136, 136, 150, 255}
	colAccent    = color.RGBA{232, 168, 72, 255}
	colAccentDim = color.RGBA{96, 72, 36, 255}
	colHover     = color.RGBA{54, 54, 64, 255}
	colDanger    = color.RGBA{206, 88, 72, 255}
	colGrid      = color.RGBA{255, 255, 255, 28}
)

// ui carries this frame's input and the little state a form needs between frames.
//
// Edges - a click, a key going down, a wheel notch, a typed rune - are collected in update() and held until
// the next draw reads them. Ebiten runs Update and Draw on their own clocks, so a press that begins and ends
// between two draws is one the widgets would otherwise never see.
type ui struct {
	mx, my                  int
	down, clicked, rightClk bool
	middle, released        bool
	wheel                   float64
	runes                   []rune
	focus                   string // the id of the field taking keystrokes
	edit                    string // its working text
	caret                   int
	tooltip                 string
	blocked                 image.Rectangle // a popup's area: widgets under it get no mouse
	popupOpen               bool

	pending pending // edges seen since the last draw
}

// pending is what happened since the widgets last looked.
type pending struct {
	clicked, released, rightClk bool
	wheel                       float64
	runes                       []rune
	keys                        map[ebiten.Key]bool // pressed, or repeating
}

// repeatKeys are the ones a held press should keep firing: moving and deleting in a text field.
var repeatKeys = []ebiten.Key{ebiten.KeyBackspace, ebiten.KeyDelete, ebiten.KeyArrowLeft, ebiten.KeyArrowRight}

// update collects this tick's edges. It must run from Update, which is the only place inpututil is accurate.
func (u *ui) update() {
	if u.pending.keys == nil {
		u.pending.keys = map[ebiten.Key]bool{}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		u.pending.clicked = true
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		u.pending.released = true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		u.pending.rightClk = true
	}
	_, w := ebiten.Wheel()
	u.pending.wheel += w
	u.pending.runes = ebiten.AppendInputChars(u.pending.runes)
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		u.pending.keys[k] = true
	}
	for _, k := range repeatKeys {
		if d := inpututil.KeyPressDuration(k); d > 25 && d%3 == 0 {
			u.pending.keys[k] = true
		}
	}
}

func (u *ui) beginFrame() {
	u.mx, u.my = ebiten.CursorPosition()
	u.down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	u.middle = ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	u.clicked, u.released, u.rightClk = u.pending.clicked, u.pending.released, u.pending.rightClk
	u.wheel = u.pending.wheel
	u.runes = append(u.runes[:0], u.pending.runes...)
	u.tooltip = ""
	u.pending.clicked, u.pending.released, u.pending.rightClk = false, false, false
	u.pending.wheel, u.pending.runes = 0, u.pending.runes[:0]
}

// endFrame drops the key edges the widgets have now had their chance at.
func (u *ui) endFrame() {
	clear(u.pending.keys)
}

// key reports whether the key went down, or repeated, since the last frame.
func (u *ui) key(k ebiten.Key) bool { return u.pending.keys[k] }

// hit reports whether the mouse is over r and not over an open popup above it.
func (u *ui) hit(r image.Rectangle) bool {
	p := image.Pt(u.mx, u.my)
	if u.popupOpen && p.In(u.blocked) {
		return false
	}
	return p.In(r)
}

// typing reports whether a text field currently has the keyboard.
func (u *ui) typing() bool { return u.focus != "" }

func rect(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+w, y+h) }

func fill(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	vector.DrawFilledRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
}

func stroke(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	vector.StrokeRect(dst, float32(r.Min.X)+0.5, float32(r.Min.Y)+0.5, float32(r.Dx())-1, float32(r.Dy())-1, 1, c, false)
}

func label(dst *ebiten.Image, x, y int, s string, c color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, face, op)
}

// labelClipped draws text that stops at w pixels, with an ellipsis when it does not fit.
func labelClipped(dst *ebiten.Image, x, y, w int, s string, c color.Color) {
	if n := w / charW; len(s) > n {
		if n < 2 {
			return
		}
		s = s[:n-1] + "..."
	}
	label(dst, x, y, s, c)
}

func textWidth(s string) int { return len(s) * charW }

// button draws a button and reports a click on it.
func (u *ui) button(dst *ebiten.Image, r image.Rectangle, s string, enabled bool) bool {
	over := enabled && u.hit(r)
	bg := colPanel
	switch {
	case !enabled:
		bg = colPanelAlt
	case over && u.down:
		bg = colAccentDim
	case over:
		bg = colHover
	}
	fill(dst, r, bg)
	stroke(dst, r, colLine)
	c := colText
	if !enabled {
		c = colTextDim
	}
	labelClipped(dst, r.Min.X+padding, r.Min.Y+(r.Dy()-lineH)/2+1, r.Dx()-2*padding, s, c)
	return over && u.clicked
}

// toggle is a button that shows whether it is on.
func (u *ui) toggle(dst *ebiten.Image, r image.Rectangle, s string, on bool) bool {
	over := u.hit(r)
	bg := colPanel
	if on {
		bg = colAccentDim
	}
	if over && !on {
		bg = colHover
	}
	fill(dst, r, bg)
	stroke(dst, r, colLine)
	c := colText
	if on {
		c = colAccent
	}
	labelClipped(dst, r.Min.X+padding, r.Min.Y+(r.Dy()-lineH)/2+1, r.Dx()-2*padding, s, c)
	return over && u.clicked
}

// checkbox is a labelled on/off box; it returns the new value.
func (u *ui) checkbox(dst *ebiten.Image, r image.Rectangle, s string, on bool) bool {
	box := rect(r.Min.X, r.Min.Y+1, lineH-2, lineH-2)
	fill(dst, box, colPanelAlt)
	stroke(dst, box, colLine)
	if on {
		fill(dst, box.Inset(3), colAccent)
	}
	labelClipped(dst, box.Max.X+padding, r.Min.Y+1, r.Dx()-box.Dx()-2*padding, s, colText)
	if u.hit(r) && u.clicked {
		return !on
	}
	return on
}

// field is a one-line text box. It returns the value, and true on the frame the edit is committed.
func (u *ui) field(dst *ebiten.Image, r image.Rectangle, id, value string) (string, bool) {
	active := u.focus == id
	if u.hit(r) && u.clicked && !active {
		u.focus, u.edit, u.caret = id, value, len(value)
		active = true
	} else if u.clicked && active && !u.hit(r) {
		u.focus = ""
		return u.edit, true // clicking away keeps what was typed
	}
	fill(dst, r, colPanelAlt)
	stroke(dst, r, map[bool]color.RGBA{true: colAccent, false: colLine}[active])
	shown := value
	if active {
		shown = u.edit
		u.editKeys()
		if u.key(ebiten.KeyEnter) || u.key(ebiten.KeyNumpadEnter) || u.key(ebiten.KeyTab) {
			u.focus = ""
			return u.edit, true
		}
		if u.key(ebiten.KeyEscape) {
			u.focus = ""
			return value, false
		}
	}
	// scroll the text so the caret stays visible
	vis := (r.Dx() - 2*padding) / charW
	start := 0
	if active && u.caret > vis {
		start = u.caret - vis
	}
	if start > len(shown) {
		start = len(shown)
	}
	end := min(len(shown), start+vis)
	labelClipped(dst, r.Min.X+padding, r.Min.Y+(r.Dy()-lineH)/2+1, r.Dx()-2*padding, strings.ReplaceAll(shown[start:end], "\n", "\\n"), colText)
	if active {
		cx := r.Min.X + padding + (u.caret-start)*charW
		vector.StrokeLine(dst, float32(cx), float32(r.Min.Y+3), float32(cx), float32(r.Max.Y-3), 1, colAccent, false)
	}
	return shown, false
}

// editKeys applies this frame's typing to the focused field's working text.
func (u *ui) editKeys() {
	if len(u.runes) > 0 {
		u.edit = u.edit[:u.caret] + string(u.runes) + u.edit[u.caret:]
		u.caret += len(string(u.runes))
	}
	if u.key(ebiten.KeyBackspace) && u.caret > 0 {
		u.edit = u.edit[:u.caret-1] + u.edit[u.caret:]
		u.caret--
	}
	if u.key(ebiten.KeyDelete) && u.caret < len(u.edit) {
		u.edit = u.edit[:u.caret] + u.edit[u.caret+1:]
	}
	if u.key(ebiten.KeyArrowLeft) && u.caret > 0 {
		u.caret--
	}
	if u.key(ebiten.KeyArrowRight) && u.caret < len(u.edit) {
		u.caret++
	}
	if u.key(ebiten.KeyHome) {
		u.caret = 0
	}
	if u.key(ebiten.KeyEnd) {
		u.caret = len(u.edit)
	}
}

// scroller keeps one list's scroll offset between frames.
type scroller struct{ off float64 }

// clamp keeps the offset inside a list of that many pixels shown in a box that tall.
func (s *scroller) clamp(content, view int) int {
	m := float64(max(0, content-view))
	s.off = min(max(s.off, 0), m)
	return int(s.off)
}

// scrollbar draws the position indicator for a list and lets the wheel move it.
func (u *ui) scrollbar(dst *ebiten.Image, r image.Rectangle, s *scroller, content int) {
	if u.hit(r) && u.wheel != 0 {
		s.off -= u.wheel * lineH * 3
	}
	if content <= r.Dy() {
		return
	}
	bar := rect(r.Max.X-4, r.Min.Y, 4, r.Dy())
	fill(dst, bar, colPanelAlt)
	h := max(12, r.Dy()*r.Dy()/content)
	y := r.Min.Y + int(s.off)*(r.Dy()-h)/max(1, content-r.Dy())
	fill(dst, rect(bar.Min.X, y, 4, h), colLine)
}

// tip records the hint to show at the bottom while the mouse is over r.
func (u *ui) tip(r image.Rectangle, s string) {
	if u.hit(r) {
		u.tooltip = s
	}
}

func ctrl() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
}

func shiftHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift)
}

func colorRGBA(r, g, b, a uint8) color.RGBA { return color.RGBA{r, g, b, a} }

// wrap breaks text into lines of at most cols characters, on spaces where it can.
func wrap(s string, cols int) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, w := range strings.Fields(para) {
			switch {
			case line == "":
				line = w
			case len(line)+1+len(w) <= cols:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
	}
	return out
}

// labelWrapped draws text over as many lines as it needs, and returns the y below it.
func labelWrapped(dst *ebiten.Image, x, y, w int, s string, c color.Color) int {
	for _, line := range wrap(s, max(1, w/charW)) {
		label(dst, x, y, line, c)
		y += lineH
	}
	return y
}
