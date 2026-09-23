package main

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// A megatile has no art here: the game never draws Diablo's own tiles, it reads the pieces' sol flags and
// paints a wall or a floor. So the editor shows exactly that, a quarter at a time, which is the thing being
// designed.
var (
	colFloor   = color.RGBA{58, 62, 70, 255}
	colSolid   = color.RGBA{150, 120, 88, 255}
	colBlocks  = color.RGBA{112, 80, 140, 255}
	colMissile = color.RGBA{70, 100, 140, 255}
	colNoMega  = color.RGBA{30, 30, 34, 255}
)

// quarterColor is what one quarter of a megatile blocks.
func quarterColor(flags byte) color.RGBA {
	switch {
	case flags&1 != 0 && flags&2 != 0:
		return colSolid
	case flags&1 != 0:
		return color.RGBA{128, 104, 76, 255}
	case flags&2 != 0:
		return colBlocks
	case flags&4 != 0:
		return colMissile
	}
	return colFloor
}

// layerColor is how each overlay's numbers show.
var layerColor = map[string]color.RGBA{
	"transparency": {120, 200, 255, 255},
	"items":        {240, 220, 120, 255},
	"monsters":     {240, 120, 120, 255},
	"objects":      {150, 230, 150, 255},
}

func (p *pieceEditor) screenOf(c image.Rectangle, x, y int) (float64, float64) {
	s := p.cellPx()
	return float64(c.Min.X) - p.cam.x + float64(x)*s, float64(c.Min.Y) - p.cam.y + float64(y)*s
}

// cellAt is the megatile under the pointer; quarterAt is the double-resolution cell under it.
func (p *pieceEditor) cellAt(c image.Rectangle, mx, my int) (int, int) {
	s := p.cellPx()
	return int(floor((float64(mx-c.Min.X) + p.cam.x) / s)), int(floor((float64(my-c.Min.Y) + p.cam.y) / s))
}

func (p *pieceEditor) quarterAt(c image.Rectangle, mx, my int) (int, int) {
	s := p.cellPx() / 2
	return int(floor((float64(mx-c.Min.X) + p.cam.x) / s)), int(floor((float64(my-c.Min.Y) + p.cam.y) / s))
}

func (p *pieceEditor) drawCanvas(a *app, dst *ebiten.Image, c image.Rectangle) {
	u := &a.u
	fill(dst, c, colBG)
	if p.cur == nil {
		msg := "no set piece open - pick one on the left"
		if len(p.files) == 0 {
			msg = "this mod ships no Diablo set pieces: they live in its data/levels/l1..l4"
		}
		labelWrapped(dst, c.Min.X+16, c.Min.Y+16, c.Dx()-32, msg, colTextDim)
		return
	}
	over := u.hit(c) && p.popup == ""
	p.zoomAndPan(u, c, over)
	view := dst.SubImage(c).(*ebiten.Image)
	s := p.cellPx()
	t := p.table()

	for y := 0; y < p.cur.h; y++ {
		for x := 0; x < p.cur.w; x++ {
			sx, sy := p.screenOf(c, x, y)
			if sx > float64(c.Max.X) || sy > float64(c.Max.Y) || sx+s < float64(c.Min.X) || sy+s < float64(c.Min.Y) {
				continue
			}
			mega := p.cur.tileAt(x, y)
			if mega == 0 || t == nil {
				fill(view, image.Rect(int(sx), int(sy), int(sx+s), int(sy+s)), colNoMega)
			} else {
				q := t.quarters(mega)
				// nw ne / sw se, as the generator lays them out
				for i, piece := range q {
					qx, qy := sx+float64(i%2)*s/2, sy+float64(i/2)*s/2
					fill(view, image.Rect(int(qx), int(qy), int(qx+s/2)+1, int(qy+s/2)+1), quarterColor(t.pieceFlags(piece)))
				}
			}
			vector.StrokeRect(view, float32(sx), float32(sy), float32(s), float32(s), 1, colLine, false)
			if p.showIDs && s >= 26 && mega != 0 {
				label(view, int(sx)+3, int(sy)+2, strconv.Itoa(mega), color.RGBA{230, 230, 240, 160})
			}
		}
	}

	// the double-resolution overlays, drawn over the megatiles they sit on
	for _, name := range []string{"items", "monsters", "objects"} {
		for pt, v := range p.cur.layers[name] {
			if v == 0 {
				continue
			}
			p.drawQuarterMark(view, c, pt.x, pt.y, strconv.Itoa(v), layerColor[name], name == pieceLayers[p.layer])
		}
	}
	for y := 0; y < 2*p.cur.h; y++ {
		for x := 0; x < 2*p.cur.w; x++ {
			if v := p.cur.transAt(x, y); v > 0 {
				p.drawQuarterMark(view, c, x, y, strconv.Itoa(v), layerColor["transparency"], pieceLayers[p.layer] == "transparency")
			}
		}
	}

	if !over {
		return
	}
	layer := pieceLayers[p.layer]
	if layer == "tiles" {
		x, y := p.cellAt(c, u.mx, u.my)
		sx, sy := p.screenOf(c, x, y)
		vector.StrokeRect(view, float32(sx), float32(sy), float32(s), float32(s), 2, colAccent, false)
		a.status = fmt.Sprintf("%s  %dx%d   megatile %d,%d = %s   brush %d   %s",
			p.cur.name(), p.cur.w, p.cur.h, x, y, megaLabel(p.cur.tileAt(x, y)), p.brush, toolHelp[min(p.tool, toolErase)])
		p.paintTiles(a, x, y)
		return
	}
	qx, qy := p.quarterAt(c, u.mx, u.my)
	sx, sy := p.screenOf(c, 0, 0)
	vector.StrokeRect(view, float32(sx+float64(qx)*s/2), float32(sy+float64(qy)*s/2), float32(s/2), float32(s/2), 2, colAccent, false)
	a.status = fmt.Sprintf("%s   %s at %d,%d = %d   value %d   %s", p.cur.name(), layer, qx, qy, p.valueAt(layer, qx, qy), p.brush, layerHelp[layer])
	p.paintQuarters(a, layer, qx, qy)
}

func megaLabel(v int) string {
	if v == 0 {
		return "none"
	}
	return strconv.Itoa(v)
}

func (p *pieceEditor) drawQuarterMark(view *ebiten.Image, c image.Rectangle, x, y int, s string, col color.RGBA, active bool) {
	q := p.cellPx() / 2
	sx, sy := p.screenOf(c, 0, 0)
	sx, sy = sx+float64(x)*q, sy+float64(y)*q
	a := uint8(70)
	if active {
		a = 150
	}
	fill(view, image.Rect(int(sx)+1, int(sy)+1, int(sx+q)-1, int(sy+q)-1), color.RGBA{col.R, col.G, col.B, a})
	if p.showIDs && q >= 14 {
		label(view, int(sx)+2, int(sy)+int(q/2)-6, s, color.Black)
	}
}

func (p *pieceEditor) valueAt(layer string, x, y int) int {
	if layer == "transparency" {
		return p.cur.transAt(x, y)
	}
	return p.cur.layers[layer][point{x, y}]
}

func (p *pieceEditor) setValue(layer string, x, y, v int) {
	if x < 0 || y < 0 || x >= 2*p.cur.w || y >= 2*p.cur.h {
		return
	}
	if layer == "transparency" {
		p.cur.setTrans(x, y, v)
		return
	}
	if v == 0 {
		delete(p.cur.layers[layer], point{x, y})
		return
	}
	p.cur.layers[layer][point{x, y}] = v
}

func (p *pieceEditor) paintTiles(a *app, x, y int) {
	u := &a.u
	if u.middle {
		return
	}
	switch p.tool {
	case toolBrush, toolErase:
		v := p.brush
		if p.tool == toolErase {
			v = 0
		}
		if u.clicked {
			p.snapshot()
		}
		if u.down && p.cur.tileAt(x, y) >= 0 && p.cur.tileAt(x, y) != v {
			p.cur.setTile(x, y, v)
			p.dirty = true
		}
	case toolRect:
		if u.clicked {
			p.rectA, p.rectOn = image.Pt(x, y), true
		}
		if p.rectOn && u.released {
			p.snapshot()
			r := normalize(p.rectA, image.Pt(x, y))
			for j := r.Min.Y; j < r.Max.Y; j++ {
				for i := r.Min.X; i < r.Max.X; i++ {
					p.cur.setTile(i, j, p.brush)
				}
			}
			p.rectOn, p.dirty = false, true
		}
	case toolFill:
		if u.clicked {
			p.snapshot()
			p.flood(x, y, p.brush)
			p.dirty = true
		}
	case toolPick:
		if u.down {
			if v := p.cur.tileAt(x, y); v > 0 {
				p.brush = v
			}
		}
	}
	if u.rightClk {
		p.snapshot()
		p.cur.setTile(x, y, 0)
		p.dirty = true
	}
}

func (p *pieceEditor) paintQuarters(a *app, layer string, x, y int) {
	u := &a.u
	if u.middle {
		return
	}
	switch p.tool {
	case toolBrush, toolErase, toolRect, toolFill:
		v := p.brush
		if p.tool == toolErase {
			v = 0
		}
		if u.clicked {
			p.snapshot()
		}
		if u.down && p.valueAt(layer, x, y) != v {
			p.setValue(layer, x, y, v)
			p.dirty = true
		}
	case toolPick:
		if u.down {
			p.brush = p.valueAt(layer, x, y)
		}
	}
	if u.rightClk {
		p.snapshot()
		p.setValue(layer, x, y, 0)
		p.dirty = true
	}
}

func (p *pieceEditor) flood(x, y, to int) {
	from := p.cur.tileAt(x, y)
	if from == to || from < 0 {
		return
	}
	queue := []image.Point{{X: x, Y: y}}
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if p.cur.tileAt(q.X, q.Y) != from {
			continue
		}
		p.cur.setTile(q.X, q.Y, to)
		queue = append(queue, image.Pt(q.X+1, q.Y), image.Pt(q.X-1, q.Y), image.Pt(q.X, q.Y+1), image.Pt(q.X, q.Y-1))
	}
}

func (p *pieceEditor) zoomAndPan(u *ui, c image.Rectangle, over bool) {
	if over && u.wheel != 0 {
		before := p.cellPx()
		p.cam.zoom = min(max(p.cam.zoom*(1+u.wheel*0.12), 0.5), 10)
		after := p.cellPx()
		fx := (float64(u.mx-c.Min.X) + p.cam.x) / before
		fy := (float64(u.my-c.Min.Y) + p.cam.y) / before
		p.cam.x, p.cam.y = fx*after-float64(u.mx-c.Min.X), fy*after-float64(u.my-c.Min.Y)
	}
	if u.middle || (ebiten.IsKeyPressed(ebiten.KeySpace) && u.down) {
		dx, dy := ebiten.CursorPosition()
		if p.panning {
			p.cam.x -= float64(dx - p.panX)
			p.cam.y -= float64(dy - p.panY)
		}
		p.panX, p.panY, p.panning = dx, dy, true
	} else {
		p.panning = false
	}
}
