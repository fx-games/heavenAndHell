package gui

import (
	"image/color"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/renderer"
	"rpg/util"

	"github.com/hajimehoshi/ebiten/v2"
)

// InfoWindow is the right-click description of an actor: the same titled blocks of label/value rows the
// character sheet draws, in one column, or two when the blocks would not fit the screen's height.
type InfoWindow struct {
	u           *UIController
	title       string
	cols        [][]common.D1SheetSection
	shouldClose bool
	topLeft     geometry.Point
	size        geometry.Point
	lineH       int
	labelW      int
	colW        int
}

const (
	infoPad    = 16 // inside the border, around the columns
	infoColGap = 24
)

// OpenInfoWindow puts an actor description up. Any click or ESC takes it down again.
func (u *UIController) OpenInfoWindow(title string, sections []common.D1SheetSection) {
	if len(sections) == 0 {
		return
	}
	w := &InfoWindow{u: u, title: title, cols: [][]common.D1SheetSection{sections}}
	w.Layout()
	u.PushWidget(w)
}

// sectionLines is how many text lines a block of sections takes: a title line and a half-line gap per section,
// plus its rows and notes.
func sectionLines(sections []common.D1SheetSection) int {
	lines := 0
	for _, sec := range sections {
		lines += len(sec.Rows) + len(sec.Notes) + 2
	}
	return lines
}

func (w *InfoWindow) Layout() {
	r := w.u.GetRenderer()
	screen := w.u.GetScreenSize()
	_, h := r.MeasureString("M")
	w.lineH = int(h) + 3

	// notes are prose, so they are wrapped to a third of the screen before anything is measured
	noteWidth := screen.X / 3
	sections := append([]common.D1SheetSection(nil), w.cols[0]...)
	for i, sec := range sections {
		if len(sec.Notes) == 0 {
			continue
		}
		var wrapped []string
		for _, line := range util.WrapLabelText(util.ToLabelText(sec.Notes), noteWidth, r.MeasureString) {
			wrapped = append(wrapped, line.Text)
		}
		sections[i].Notes = wrapped
	}

	widest := func(dst *int, text string, gutter int) {
		width, _ := r.MeasureString(text)
		*dst = max(*dst, int(width)+gutter)
	}
	w.labelW, w.colW = 0, 0
	valueW := 0
	for _, sec := range sections {
		widest(&w.colW, sec.Title, 0)
		for _, row := range sec.Rows {
			widest(&w.labelW, row[0], 12)
			widest(&valueW, row[1], 0)
		}
		for _, note := range sec.Notes {
			widest(&w.colW, note, 0)
		}
	}
	w.colW = max(w.colW, w.labelW+valueW)

	// one column unless it would run off the screen, then split the sections in half by line count
	w.cols = [][]common.D1SheetSection{sections}
	lines := sectionLines(sections)
	if (lines+2)*w.lineH > screen.Y*17/20 {
		half, count := lines/2, 0
		for i, sec := range sections {
			count += len(sec.Rows) + len(sec.Notes) + 2
			if count >= half && i < len(sections)-1 {
				w.cols = [][]common.D1SheetSection{sections[:i+1], sections[i+1:]}
				break
			}
		}
		lines = max(sectionLines(w.cols[0]), sectionLines(w.cols[1]))
	}

	w.size = geometry.Point{X: len(w.cols)*w.colW + (len(w.cols)-1)*infoColGap, Y: (lines + 2) * w.lineH}
	w.topLeft = screen.Sub(w.size).Div(2)
}

func (w *InfoWindow) Draw() {
	r := w.u.GetRenderer()
	pad := geometry.Point{X: 2 * infoPad, Y: 2 * infoPad}
	r.DrawColoredRect(w.topLeft.Sub(pad.Div(2)), w.size.Add(pad), color.RGBA{R: 12, G: 9, B: 7, A: 240})
	r.DrawDefaultBorder(w.topLeft.Sub(pad.Div(2)), w.size.Add(pad))

	y := w.topLeft.Y + w.lineH
	r.DrawTTFOnScreen(float64(w.topLeft.X), float64(y), w.title, defs.HeaderTextColor)
	for i, sections := range w.cols {
		drawSectionBlock(r, sections, w.topLeft.X+i*(w.colW+infoColGap), y, w.labelW, w.lineH)
	}
}

// drawSectionBlock draws titled sections as a column of label/value rows with full-width notes underneath, and
// returns the y it stopped at. The character sheet and the info window share it, so both read the same.
func drawSectionBlock(r *renderer.TileRenderer, sections []common.D1SheetSection, x, y, labelW, lineH int) int {
	for _, sec := range sections {
		if len(sec.Rows) == 0 && len(sec.Notes) == 0 {
			continue
		}
		y += lineH
		r.DrawTTFOnScreen(float64(x), float64(y), sec.Title, sheetColor(sheetHeaderColors, sec.Title, defs.HeaderTextColor))
		for _, row := range sec.Rows {
			y += lineH
			r.DrawTTFOnScreen(float64(x), float64(y), row[0], defs.DarkTextColor)
			r.DrawTTFOnScreen(float64(x+labelW), float64(y), row[1], sheetColor(sheetValueColors, row[0], defs.LightTextColor))
		}
		for _, note := range sec.Notes {
			y += lineH
			r.DrawTTFOnScreen(float64(x), float64(y), note, defs.LightTextColor)
		}
		y += lineH / 3
	}
	return y
}

func (w *InfoWindow) OnCommand(command CommandType) bool {
	switch command {
	case PlayerCommandConfirm, PlayerCommandCancel:
		w.shouldClose = true
	}
	return true
}

func (w *InfoWindow) OnMouseClicked(button ebiten.MouseButton, x int, y int) bool {
	w.shouldClose = true
	return true
}

func (w *InfoWindow) OnMouseMoved(x int, y int) (bool, Tooltip)  { return true, NoTooltip{} }
func (w *InfoWindow) OnMouseWheel(x int, y int, dy float64) bool { return true }
func (w *InfoWindow) ShouldClose() bool                          { return w.shouldClose }
func (w *InfoWindow) IsNull() bool                               { return false }
