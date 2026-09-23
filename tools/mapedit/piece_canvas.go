package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// A megatile is drawn with the tiles the game draws it with: its map part in maps/<lt>_megatiles.hhm. A mod
// without those falls back to what each quarter blocks (sol flags), which is what the generator reads.
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
				p.drawMega(a, view, t, p.cur.levelType, mega, sx, sy, s)
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

// partTiles is the four tile ids the game draws megatile mega with: its map part in maps/<lt>_megatiles.hhm,
// picked by generator.txt's <lt>MegatileParts table. ok is false when the mod has neither.
func (p *pieceEditor) partTiles(m *mapEditor, lt string, mega int) (q [4]int, ok bool) {
	if p.parts == nil {
		p.parts = map[string][]int{}
	}
	tbl, seen := p.parts[lt]
	if !seen {
		tbl = readPartTable(filepath.Join(p.levels, "generator.txt"), lt+"MegatileParts")
		p.parts[lt] = tbl
	}
	if mega < 1 || mega > len(tbl) || m == nil || m.proj == nil {
		return q, false
	}
	for _, l := range m.proj.Levels {
		if l.Name != lt+"_megatiles" {
			continue
		}
		slot := tbl[mega-1] - 1
		x, y := slot%16*3, slot/16*3
		return [4]int{l.TileAt(x, y), l.TileAt(x+1, y), l.TileAt(x, y+1), l.TileAt(x+1, y+1)}, true
	}
	return q, false
}

// readPartTable reads one [table name] of generator.txt: whitespace-separated numbers up to the next section.
func readPartTable(path, name string) []int {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []int
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			if in {
				break
			}
			in = line == "[table "+name+"]"
			continue
		}
		if !in || strings.HasPrefix(line, "#") {
			continue
		}
		for _, f := range strings.Fields(line) {
			if n, err := strconv.Atoi(f); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

// drawMega draws megatile mega at (x, y), size pixels square: the game's own tiles when the mod draws its map
// parts, else each quarter's sol flags.
func (p *pieceEditor) drawMega(a *app, dst *ebiten.Image, t *megaTable, lt string, mega int, x, y, size float64) {
	m := a.maps
	if q, ok := p.partTiles(m, lt, mega); ok {
		g := float64(m.proj.Tileset.GridSize)
		for i, id := range q {
			if pic := int(m.drawn(id).Icon); id >= 0 && pic < len(m.tile) {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(size/2/g, size/2/g)
				op.GeoM.Translate(x+float64(i%2)*size/2, y+float64(i/2)*size/2)
				dst.DrawImage(m.tile[pic], op)
			}
		}
		return
	}
	for i, piece := range t.quarters(mega) { // nw ne / sw se, as the generator lays them out
		qx, qy := x+float64(i%2)*size/2, y+float64(i/2)*size/2
		fill(dst, image.Rect(int(qx), int(qy), int(qx+size/2)+1, int(qy+size/2)+1), quarterColor(t.pieceFlags(piece)))
	}
}

// megaNames is the tile names megatile mega is drawn with, for tooltips.
func (p *pieceEditor) megaNames(a *app, lt string, mega int) string {
	q, ok := p.partTiles(a.maps, lt, mega)
	if !ok {
		return ""
	}
	var names []string
	for _, id := range q {
		if n := a.maps.drawn(id).Name; n != "" && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}
