package main

import (
	"fmt"
	"image"
	"image/color"

	"rpg/src/hhm"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// entityColor is how each kind shows on the canvas, so a map reads at a glance.
var entityColor = map[string]color.RGBA{
	"NPC":            {236, 196, 96, 255},
	"Sign":           {148, 196, 236, 255},
	"Door":           {196, 140, 92, 255},
	"Stash":          {168, 168, 200, 255},
	"Shrine":         {180, 120, 220, 255},
	"Location":       {112, 216, 160, 255},
	"POI":            {216, 112, 168, 255},
	"NorthConnector": {96, 168, 255, 255},
	"SouthConnector": {96, 168, 255, 255},
	"EastConnector":  {96, 168, 255, 255},
	"WestConnector":  {96, 168, 255, 255},
}

func kindColor(kind string) color.RGBA {
	if c, ok := entityColor[kind]; ok {
		return c
	}
	return colAccent
}

// short is the tag drawn on an entity when there is no room for its name.
func short(kind string) string {
	switch {
	case len(kind) > 9 && kind[len(kind)-9:] == "Connector":
		return kind[:1] + ">"
	case len(kind) >= 2:
		return kind[:2]
	}
	return kind
}

func (m *mapEditor) screenOf(c image.Rectangle, x, y int) (float64, float64) {
	p := m.cellPx()
	return float64(c.Min.X) - m.cam.x + float64(x)*p, float64(c.Min.Y) - m.cam.y + float64(y)*p
}

func (m *mapEditor) cellAt(c image.Rectangle, mx, my int) (int, int) {
	p := m.cellPx()
	fx := (float64(mx-c.Min.X) + m.cam.x) / p
	fy := (float64(my-c.Min.Y) + m.cam.y) / p
	return int(floor(fx)), int(floor(fy))
}

func floor(f float64) float64 {
	if f < 0 && f != float64(int(f)) {
		return float64(int(f) - 1)
	}
	return float64(int(f))
}

func (m *mapEditor) drawCanvas(a *app, dst *ebiten.Image, c image.Rectangle) {
	u := &a.u
	fill(dst, c, colBG)
	if m.cur == nil {
		labelClipped(dst, c.Min.X+16, c.Min.Y+16, c.Dx(), "no level open - pick one on the left, or make a new one", colTextDim)
		return
	}
	over := u.hit(c) && m.popup == ""
	m.zoomAndPan(u, c, over)

	view := dst.SubImage(c).(*ebiten.Image)
	p := m.cellPx()

	// the level's own area, so its edges are visible against the backdrop
	ox, oy := m.screenOf(c, 0, 0)
	fill(view, image.Rect(int(ox), int(oy), int(ox+float64(m.cur.Width)*p), int(oy+float64(m.cur.Height)*p)), colPanelAlt)

	x0, y0 := m.cellAt(c, c.Min.X, c.Min.Y)
	x1, y1 := m.cellAt(c, c.Max.X, c.Max.Y)
	for y := max(y0, 0); y <= min(y1, m.cur.Height-1); y++ {
		for x := max(x0, 0); x <= min(x1, m.cur.Width-1); x++ {
			id := m.cur.TileAt(x, y)
			sx, sy := m.screenOf(c, x, y)
			if pic := int(m.drawn(id).Icon); id >= 0 && pic < len(m.tile) {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(m.cam.zoom, m.cam.zoom)
				op.GeoM.Translate(sx, sy)
				view.DrawImage(m.tile[pic], op)
			}
			if m.showFlags && id >= 0 {
				if c := flagTint(m.drawn(id).Flags); c.A > 0 {
					fill(view, image.Rect(int(sx), int(sy), int(sx+p), int(sy+p)), c)
				}
			}
		}
	}

	if m.showGrid && p >= 8 {
		for x := max(x0, 0); x <= min(x1+1, m.cur.Width); x++ {
			sx, _ := m.screenOf(c, x, 0)
			_, sy0 := m.screenOf(c, 0, max(y0, 0))
			_, sy1 := m.screenOf(c, 0, min(y1+1, m.cur.Height))
			vector.StrokeLine(view, float32(sx), float32(sy0), float32(sx), float32(sy1), 1, colGrid, false)
		}
		for y := max(y0, 0); y <= min(y1+1, m.cur.Height); y++ {
			_, sy := m.screenOf(c, 0, y)
			sx0, _ := m.screenOf(c, max(x0, 0), 0)
			sx1, _ := m.screenOf(c, min(x1+1, m.cur.Width), 0)
			vector.StrokeLine(view, float32(sx0), float32(sy), float32(sx1), float32(sy), 1, colGrid, false)
		}
	}

	for _, e := range m.cur.Entities {
		m.drawEntity(view, c, e, e == m.sel)
	}

	mx, my := m.cellAt(c, u.mx, u.my)
	if m.rectOn {
		r := normalize(m.rectA, image.Pt(mx, my))
		sx, sy := m.screenOf(c, r.Min.X, r.Min.Y)
		vector.StrokeRect(view, float32(sx), float32(sy), float32(float64(r.Dx())*p), float32(float64(r.Dy())*p), 2, colAccent, false)
	}
	if over {
		sx, sy := m.screenOf(c, mx, my)
		vector.StrokeRect(view, float32(sx), float32(sy), float32(p), float32(p), 2, colAccent, false)
		a.status = fmt.Sprintf("%s  %dx%d   cell %d,%d   tile %s   %s", m.cur.Name, m.cur.Width, m.cur.Height,
			mx, my, tileLabel(m.cur.TileAt(mx, my)), toolHelp[m.tool])
		m.handleMouse(a, c, mx, my)
	}
}

func tileLabel(id int) string {
	if id == hhm.Empty {
		return "empty"
	}
	return fmt.Sprint(id)
}

// flagTint is the colour a tile's markers paint over it in the flags overlay.
func flagTint(f hhm.Flags) color.RGBA {
	switch {
	case f.Water:
		return color.RGBA{40, 110, 220, 90}
	case f.Damaging:
		return color.RGBA{220, 70, 40, 90}
	case f.BlocksMovement && f.BlocksView:
		return color.RGBA{230, 60, 90, 70}
	case f.BlocksMovement:
		return color.RGBA{230, 150, 40, 70}
	case f.BlocksView:
		return color.RGBA{150, 60, 230, 70}
	}
	return color.RGBA{}
}

func (m *mapEditor) drawEntity(view *ebiten.Image, c image.Rectangle, e *hhm.Entity, selected bool) {
	p := m.cellPx()
	sx, sy := m.screenOf(c, e.X, e.Y)
	col := kindColor(e.Kind)
	if e.Kind == "NPC" {
		if id := e.Int("Tile"); id >= 0 && id < len(m.tile) {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(m.cam.zoom, m.cam.zoom)
			op.GeoM.Translate(sx, sy)
			view.DrawImage(m.tile[id], op)
		}
	} else {
		fill(view, image.Rect(int(sx+2), int(sy+2), int(sx+p-2), int(sy+p-2)), color.RGBA{col.R, col.G, col.B, 90})
	}
	vector.StrokeRect(view, float32(sx)+1, float32(sy)+1, float32(p)-2, float32(p)-2, 1, col, false)
	if selected {
		vector.StrokeRect(view, float32(sx)-1, float32(sy)-1, float32(p)+2, float32(p)+2, 2, color.White, false)
	}
	if !m.showNames || p < 14 {
		return
	}
	name := e.Str("Name")
	if name == "" {
		name = short(e.Kind)
	}
	label(view, int(sx)+2, int(sy)-lineH+1, name, col)
}

func normalize(a, b image.Point) image.Rectangle {
	r := image.Rect(a.X, a.Y, b.X, b.Y).Canon()
	return image.Rect(r.Min.X, r.Min.Y, r.Max.X+1, r.Max.Y+1)
}

func (m *mapEditor) zoomAndPan(u *ui, c image.Rectangle, over bool) {
	if over && u.wheel != 0 {
		before := m.cellPx()
		z := m.cam.zoom * (1 + u.wheel*0.12)
		m.cam.zoom = min(max(z, 0.5), 16)
		after := m.cellPx()
		// keep the cell under the pointer where it is
		fx := (float64(u.mx-c.Min.X) + m.cam.x) / before
		fy := (float64(u.my-c.Min.Y) + m.cam.y) / before
		m.cam.x = fx*after - float64(u.mx-c.Min.X)
		m.cam.y = fy*after - float64(u.my-c.Min.Y)
	}
	if u.middle || (ebiten.IsKeyPressed(ebiten.KeySpace) && u.down) {
		dx, dy := ebiten.CursorPosition()
		if m.panning {
			m.cam.x -= float64(dx - m.panX)
			m.cam.y -= float64(dy - m.panY)
		}
		m.panX, m.panY, m.panning = dx, dy, true
	} else {
		m.panning = false
	}
}
