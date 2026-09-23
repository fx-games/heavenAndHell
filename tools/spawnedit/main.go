// spawnedit assigns the mod's randomly rolled monsters to its dungeon levels: drag a monster from the right onto
// a level, click it for more shares of the level's rolls, right-click it off again. It edits levels.rec
// (monster lines, enemy_percent) and enemies.rec (available, out_of_depth_chance) in place; on a D1 level,
// which rolls by 1.09's rules, it edits dlvl/max_dlvl instead. A development tool, kept out of the game binary.
//
//	cd tools/spawnedit && go run . -mod heavenandhell
package main

import (
	"cmp"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"

	"rpg/files"
	"rpg/src/defs"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/recfile"
)

const (
	tile   = 8  // the sheet's grid
	rowH   = 90 // a level row
	enemyH = 20 // an enemy row
	cellW  = 14 // one depth in the enemy chart
	top    = 24 // header
	status = 20
)

var (
	face    = text.NewGoXFace(basicfont.Face7x13)
	colBG   = color.RGBA{24, 24, 28, 255}
	colRow  = color.RGBA{36, 36, 42, 255}
	colHov  = color.RGBA{54, 54, 64, 255}
	colLine = color.RGBA{58, 58, 68, 255}
	colText = color.RGBA{214, 214, 222, 255}
	colDim  = color.RGBA{136, 136, 150, 255}
	colOn   = color.RGBA{232, 168, 72, 255}
	colBad  = color.RGBA{206, 88, 72, 255}
	colOOD  = color.RGBA{110, 84, 50, 255}
)

// hot is a screen area the last frame drew, and what the mouse does to it.
type hot struct {
	r           image.Rectangle
	left, right func()
	drag, drop  int // an enemy to pick up / a level to drop on; -1 for neither
	tip         string
	cell        string // a matrix square: a drag sets every one it crosses to what the first one became
	on          bool   // the square's state
}

type app struct {
	enemiesPath, levelsPath string
	enemies                 []*enemy
	levels                  []*level
	depths                  []int // every level's depth, sorted, once each
	room                    map[string]*ebiten.Image
	sheet                   *ebiten.Image
	colors                  map[string]color.RGBA

	hots             []hot
	painted          map[string]bool // the squares this drag already set; nil when not dragging over the matrix
	paintTo          bool            // what this drag sets them to: the opposite of the square it started on
	dragging         int
	scrollL, scrollR int
	msg              string
	dirty            bool
	w, h             int
}

func main() {
	mod := flag.String("mod", "heavenandhell", "the mod, a name under files/mods")
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		log.Fatal(err)
	}
	files.SetFileSystem(os.DirFS(filepath.Join(root, "files")).(files.FileSystem))
	files.SelectMod(*mod)
	if !files.ModAvailable(*mod) {
		log.Fatalf("no mod %q under files/mods", *mod)
	}
	if err := defs.LoadIcons(); err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join(root, "files", "mods", *mod)
	a := &app{enemiesPath: filepath.Join(dir, "data", "enemies.rec"), levelsPath: filepath.Join(dir, "data", "levels.rec"),
		dragging: -1, room: map[string]*ebiten.Image{}, colors: map[string]color.RGBA{}, msg: "editing " + dir}
	if a.sheet, _, err = ebitenutil.NewImageFromFile(filepath.Join(dir, "classic_roguelike.png")); err != nil {
		log.Fatal(err)
	}
	if err := a.load(); err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(1400, 900)
	ebiten.SetWindowTitle("spawnedit - " + *mod)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(a); err != nil {
		log.Fatal(err)
	}
}

func repoRoot() (string, error) {
	dir, _ := filepath.Abs(".")
	for {
		if _, err := os.Stat(filepath.Join(dir, "files", "mods")); err == nil {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("run this from inside the repo")
		}
		dir = filepath.Dir(dir)
	}
}

func (a *app) load() error {
	mons, err := defs.ReadMonsters(files.GetFileSystem())
	if err != nil {
		return err
	}
	for _, m := range mons {
		icon, _ := defs.IconOK(m.Icon)
		a.enemies = append(a.enemies, &enemy{id: m.ID, name: m.Name, icon: icon, min: m.MinDLvl, max: m.MaxDLvl, avail: m.Avail,
			wgt: m.SpawnWeight, ood: m.OutOfDepthChance, oodMin: m.MinLevelForOoD, unique: slices.Contains(m.Tags, "unique"), danger: m.Exp})
		if m.Color.A == 0 { // no color field: still visible in the share bar
			m.Color = colDim
		}
		a.colors[m.ID] = m.Color
	}
	f, err := os.Open(a.levelsPath)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, r := range recfile.Read(f) {
		if town, _ := dungeon.Field(r, "town"); town == "true" {
			continue
		}
		l := &level{}
		l.id, _ = dungeon.Field(r, "map")
		l.name, _ = dungeon.Field(r, "name")
		l.name = strings.ReplaceAll(l.name, `\n`, " - ")
		l.tiles, _ = dungeon.Field(r, "tiles")
		l.depth, _ = dungeon.Int(r, "depth", 0)
		l.pct, _ = dungeon.Int(r, "enemy_percent", 0)
		for _, v := range dungeon.FieldsNamed(r, "monster") {
			id, w, _ := strings.Cut(strings.TrimSpace(v), " ")
			n, err := strconv.Atoi(strings.TrimSpace(w))
			if err != nil {
				n = 1
			}
			l.mons = append(l.mons, dungeon.LevelMonster{ID: id, Weight: max(n, 1)})
		}
		if gen, _ := dungeon.Field(r, "generator"); strings.HasPrefix(gen, "d1_") {
			d1Rules = true // D1 places monsters by 1.09's own rules: no density, no weights
		}
		a.levels = append(a.levels, l)
		if !slices.Contains(a.depths, l.depth) {
			a.depths = append(a.depths, l.depth)
		}
	}
	slices.SortStableFunc(a.levels, func(x, y *level) int { return x.depth - y.depth })
	slices.Sort(a.depths)
	// ponytail: experience paid stands in for danger; a real threat score (hp x damage x speed) if it misleads
	slices.SortStableFunc(a.enemies, func(x, y *enemy) int { return cmp.Or(b2i(x.unique)-b2i(y.unique), x.danger-y.danger) })
	for _, l := range a.levels { // a level without a list rolls by dlvl: start its list from what it rolls now
		if !d1Rules && len(l.mons) == 0 {
			for _, e := range a.enemies {
				if !e.unique && e.spawnsAt(l.depth, a.deepest()) {
					l.mons = append(l.mons, dungeon.LevelMonster{ID: e.id, Weight: e.wgt})
				}
			}
		}
	}
	return nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// here is who level l rolls, at what weight.
func (a *app) here(l *level) []dungeon.LevelMonster {
	if !d1Rules {
		return l.mons
	}
	var out []dungeon.LevelMonster
	for _, e := range a.enemies {
		if !e.unique && e.spawnsAt(l.depth, a.deepest()) {
			out = append(out, dungeon.LevelMonster{ID: e.id, Weight: 1})
		}
	}
	return out
}

// onDepth reports whether some level at depth d rolls e.
func (a *app) onDepth(e *enemy, d int) bool {
	for _, l := range a.levels {
		if l.depth == d && (d1Rules && e.spawnsAt(d, a.deepest()) || l.weight(e.id) > 0) {
			return true
		}
	}
	return false
}

// toggle puts e on, or takes it off, every level at depth d.
func (a *app) toggle(e *enemy, d int, on bool) {
	if d1Rules {
		if !on {
			e.add(d, a.deepest())
		} else if err := e.remove(d, a.deepest()); err != nil {
			a.msg = err.Error()
			return
		}
	} else {
		for _, l := range a.levels {
			if l.depth == d {
				l.setWeight(e.id, b2i(!on))
			}
		}
		if !on {
			e.avail = 1
		}
	}
	a.touch("%s on depth %d: %v", e.name, d, !on)
}

func (a *app) enemyByID(id string) *enemy {
	for _, e := range a.enemies {
		if e.id == id {
			return e
		}
	}
	return nil
}

func (a *app) deepest() int { return a.depths[len(a.depths)-1] }

func (a *app) Layout(w, h int) (int, int) { a.w, a.h = w, h; return w, h }

func (a *app) Update() error {
	mx, my := ebiten.CursorPosition()
	p := image.Pt(mx, my)
	at := func(ok func(hot) bool) *hot {
		for i := len(a.hots) - 1; i >= 0; i-- {
			if p.In(a.hots[i].r) && ok(a.hots[i]) {
				return &a.hots[i]
			}
		}
		return nil
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if h := at(func(h hot) bool { return h.drag >= 0 || h.left != nil }); h != nil {
			if h.drag >= 0 {
				a.dragging = h.drag
			} else if h.cell != "" {
				a.painted, a.paintTo = map[string]bool{}, !h.on
			} else {
				h.left()
			}
		}
	}
	if a.painted != nil {
		if h := at(func(h hot) bool { return h.cell != "" }); h != nil && !a.painted[h.cell] {
			a.painted[h.cell] = true
			if h.on != a.paintTo {
				h.left()
			}
		}
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			a.painted = nil
		}
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && a.dragging >= 0 {
		if h := at(func(h hot) bool { return h.drop >= 0 }); h != nil {
			l, e := a.levels[h.drop], a.enemies[a.dragging]
			if d1Rules {
				e.add(l.depth, a.deepest())
			} else {
				l.setWeight(e.id, max(1, l.weight(e.id)))
				e.avail = 1
			}
			a.touch("%s now spawns on %s", e.name, l.id)
		}
		a.dragging = -1
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if h := at(func(h hot) bool { return h.right != nil }); h != nil {
			h.right()
		}
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		if mx < a.w*55/100 {
			a.scrollL = max(0, a.scrollL-int(wy*40))
		} else {
			a.scrollR = max(0, a.scrollR-int(wy*40))
		}
	}
	if (ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyControl)) && inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if err := save(a.enemiesPath, a.levelsPath, a.enemies, a.levels); err != nil {
			a.msg = "save failed: " + err.Error()
		} else {
			a.dirty, a.msg = false, "saved "+filepath.Base(a.enemiesPath)+" and "+filepath.Base(a.levelsPath)
		}
	}
	return nil
}

func (a *app) touch(format string, args ...any) { a.dirty, a.msg = true, fmt.Sprintf(format, args...) }

func (a *app) Draw(dst *ebiten.Image) {
	a.hots = a.hots[:0]
	dst.Fill(colBG)
	split := a.w * 55 / 100
	a.drawLevels(dst, image.Rect(0, top, split, a.h-status))
	a.drawEnemies(dst, image.Rect(split+4, top, a.w, a.h-status))

	head := "Levels: drop monsters here, right-click one to take it off"
	if !d1Rules {
		head += "; click +1 share, shift-click -1"
	}
	label(dst, 6, 5, head, colDim)
	label(dst, split+10, 5, "Monsters: drag onto a level; dark cells: out of depth", colDim)

	fill(dst, image.Rect(0, a.h-status, a.w, a.h), colRow)
	msg := a.msg
	mx, my := ebiten.CursorPosition()
	for _, h := range a.hots {
		if h.tip != "" && image.Pt(mx, my).In(h.r) {
			msg = h.tip
		}
	}
	if a.dirty {
		msg = "* unsaved (Cmd/Ctrl+S)  " + msg
	}
	label(dst, 6, a.h-status+3, msg, colText)
	if a.dragging >= 0 {
		a.icon(dst, a.enemies[a.dragging].icon, mx-8, my-8, 2)
	}
}

func (a *app) drawLevels(dst *ebiten.Image, r image.Rectangle) {
	view := dst.SubImage(r).(*ebiten.Image)
	a.scrollL = min(a.scrollL, max(0, len(a.levels)*rowH-r.Dy()))
	for i, l := range a.levels {
		y := r.Min.Y + i*rowH - a.scrollL
		row := image.Rect(r.Min.X+4, y+2, r.Max.X-4, y+rowH-2)
		if !row.Overlaps(r) {
			continue
		}
		bg := colRow
		if a.dragging >= 0 && image.Pt(ebiten.CursorPosition()).In(row) {
			bg = colHov
		}
		fill(view, row, bg)
		a.hots = append(a.hots, hot{r: row.Intersect(r), drag: -1, drop: i})
		view.DrawImage(a.roomOf(l.tiles), geo(row.Min.X+4, row.Min.Y+4, 1))
		x := row.Min.X + 124
		label(view, x, row.Min.Y+4, fmt.Sprintf("%d  %s", l.depth, l.name), colText)
		if !d1Rules {
			pct := l.pct
			if pct == 0 {
				pct = 5
			}
			label(view, x, row.Min.Y+20, fmt.Sprintf("%2d%% of floor cells", pct), colDim)
			a.button(view, image.Rect(x+140, row.Min.Y+19, x+156, row.Min.Y+33), "-", func() { l.pct = max(1, pct-1); a.touch("%s: %d%%", l.id, l.pct) })
			a.button(view, image.Rect(x+158, row.Min.Y+19, x+174, row.Min.Y+33), "+", func() { l.pct = min(100, pct+1); a.touch("%s: %d%%", l.id, l.pct) })
		}
		// who spawns here, and the share of rolls each one gets; then who may come up out of depth
		here := a.here(l)
		total := 0
		for _, m := range here {
			total += m.Weight
		}
		cx, cy := x, row.Min.Y+38
		for _, m := range here {
			e := a.enemyByID(m.ID)
			if e == nil || cx+18 > row.Max.X {
				continue // ponytail: one line of chips, the bar below still counts them all
			}
			a.icon(view, e.icon, cx, cy, 2)
			tip := e.name
			if !d1Rules {
				label(view, cx, cy+16, fmt.Sprint(m.Weight), colDim)
				tip += fmt.Sprintf(": weight %d, %d%% of this level's monsters (click +1, shift-click -1, right-click remove)", m.Weight, 100*m.Weight/total)
			}
			a.hots = append(a.hots, hot{r: image.Rect(cx, cy, cx+16, cy+16).Intersect(r), drag: -1, drop: i, tip: tip,
				left: func() {
					if d1Rules {
						return
					}
					w := l.weight(e.id) + 1
					if ebiten.IsKeyPressed(ebiten.KeyShift) {
						w = max(1, w-2)
					}
					l.setWeight(e.id, w)
					a.touch("%s on %s: weight %d", e.name, l.id, w)
				},
				right: func() {
					if !d1Rules {
						l.setWeight(e.id, 0)
					} else if err := e.remove(l.depth, a.deepest()); err != nil {
						a.msg = err.Error()
						return
					}
					a.touch("%s no longer spawns on %s", e.name, l.id)
				}})
			cx += 18
		}
		cx += 10
		for _, e := range a.enemies {
			if l.weight(e.id) > 0 || !e.oodAt(l.depth) || cx+18 > row.Max.X {
				continue
			}
			op := geo(cx, cy, 2)
			op.ColorScale.ScaleAlpha(0.35)
			a.drawIcon(view, e.icon, op)
			a.hots = append(a.hots, hot{r: image.Rect(cx, cy, cx+16, cy+16).Intersect(r), drag: -1, drop: i,
				tip: fmt.Sprintf("%s: out of depth, %g%% per roll (native from depth %d)", e.name, 100*e.ood, e.min)})
			cx += 18
		}
		if total > 0 && !d1Rules {
			bx, bw := float64(x), float64(row.Max.X-x-8)
			for _, m := range here {
				w := bw * float64(m.Weight) / float64(total)
				vector.DrawFilledRect(view, float32(bx), float32(row.Min.Y+70), float32(w)-1, 8, a.colors[m.ID], false)
				bx += w
			}
		}
		if len(here) == 0 {
			label(view, x, cy, "no monsters", colBad)
		}
	}
}

func (a *app) drawEnemies(dst *ebiten.Image, r image.Rectangle) {
	view := dst.SubImage(r).(*ebiten.Image)
	chart := r.Max.X - len(a.depths)*cellW - 8
	for k, d := range a.depths {
		label(view, chart+k*cellW, r.Min.Y, fmt.Sprint(d%100), colDim)
	}
	body := image.Rect(r.Min.X, r.Min.Y+16, r.Max.X, r.Max.Y)
	view = dst.SubImage(body).(*ebiten.Image)
	a.scrollR = min(a.scrollR, max(0, len(a.enemies)*enemyH-body.Dy()))
	for j, e := range a.enemies {
		y := body.Min.Y + j*enemyH - a.scrollR
		row := image.Rect(r.Min.X, y, r.Max.X-4, y+enemyH-2)
		if !row.Overlaps(body) {
			continue
		}
		fill(view, row, colRow)
		a.icon(view, e.icon, row.Min.X+2, row.Min.Y+1, 2)
		nameR := image.Rect(row.Min.X, row.Min.Y, chart-96, row.Max.Y)
		if e.unique { // read only: its room places it
			labelClipped(view, row.Min.X+22, row.Min.Y+3, nameR.Dx()-24, e.name+" (unique)", colDim)
			a.hots = append(a.hots, hot{r: nameR.Intersect(body), drag: -1, drop: -1, tip: fmt.Sprintf("%s: unique, placed by its room; level %d", e.id, e.min)})
			for k, d := range a.depths {
				if d == e.min {
					stroke(view, image.Rect(chart+k*cellW, row.Min.Y+3, chart+k*cellW+cellW-2, row.Max.Y-3), colDim)
				}
			}
			continue
		}
		c := colText
		if !slices.ContainsFunc(a.depths, func(d int) bool { return a.onDepth(e, d) }) {
			c = colDim
		}
		labelClipped(view, row.Min.X+22, row.Min.Y+3, nameR.Dx()-24, e.name, c)
		a.hots = append(a.hots, hot{r: nameR.Intersect(body), drag: j, drop: -1, tip: e.id + ": drag onto a level"})
		if !d1Rules {
			label(view, chart-92, row.Min.Y+3, fmt.Sprintf("ood%3d%%", int(e.ood*100+0.5)), colText)
			tip := fmt.Sprintf("%s: chance to come up above depth %d, from depth %d on", e.name, e.min, e.oodMin)
			step := func(d float64) func() {
				return func() {
					e.ood = min(1, max(0, float64(int(e.ood*100+0.5)+int(d))/100))
					a.touch("%s: out of depth %g%%", e.name, 100*e.ood)
				}
			}
			a.button(view, image.Rect(chart-34, row.Min.Y+2, chart-20, row.Max.Y-2), "-", step(-1))
			a.button(view, image.Rect(chart-18, row.Min.Y+2, chart-4, row.Max.Y-2), "+", step(1))
			a.hots[len(a.hots)-1].tip, a.hots[len(a.hots)-2].tip = tip, tip
		}
		for k, d := range a.depths {
			cell := image.Rect(chart+k*cellW, row.Min.Y+3, chart+k*cellW+cellW-2, row.Max.Y-3)
			on := a.onDepth(e, d)
			a.hots = append(a.hots, hot{r: cell.Intersect(body), drag: -1, drop: -1, tip: fmt.Sprintf("%s on depth %d: click or drag to toggle", e.name, d), cell: fmt.Sprint(e.id, "@", d), on: on,
				left: func() { a.toggle(e, d, on) }})
			switch {
			case on:
				fill(view, cell, colOn)
			case e.oodAt(d):
				fill(view, cell, colOOD)
			default:
				fill(view, cell, colLine)
			}
		}
	}
}

func (a *app) button(dst *ebiten.Image, r image.Rectangle, s string, click func()) {
	fill(dst, r, colLine)
	label(dst, r.Min.X+4, r.Min.Y, s, colText)
	a.hots = append(a.hots, hot{r: r.Intersect(dst.Bounds()), left: click, drag: -1, drop: -1})
}

// roomOf draws a small room in the level's tile set, the way the game's tilemap would dress it.
func (a *app) roomOf(tiles string) *ebiten.Image {
	if img, ok := a.room[tiles]; ok {
		return img
	}
	const w, h = 14, 10 // 112x80 at one pixel per sheet pixel
	img := ebiten.NewImage(w*tile, h*tile)
	isWall := func(p geometry.Point) bool { return p.X <= 0 || p.Y <= 0 || p.X >= w-1 || p.Y >= h-1 }
	tm, ok := defs.Tilemap(tiles, rand.New(rand.NewSource(1)), isWall)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !ok {
				fill(img, image.Rect(x*tile, y*tile, x*tile+tile, y*tile+tile), colLine)
				continue
			}
			p := geometry.Point{X: x, Y: y}
			id := tm.ChooseFloorTile(p)
			switch {
			case x == w/2 && y == h-1:
				id = tm.ClosedDoor
			case isWall(p):
				id = tm.ChooseWallTile(p)
			}
			a.icon(img, id, x*tile, y*tile, 1)
		}
	}
	a.room[tiles] = img
	return img
}

func (a *app) icon(dst *ebiten.Image, id int32, x, y int, scale float64) {
	a.drawIcon(dst, id, geo(x, y, scale))
}

func (a *app) drawIcon(dst *ebiten.Image, id int32, op *ebiten.DrawImageOptions) {
	cols := a.sheet.Bounds().Dx() / tile
	sx, sy := int(id)%cols*tile, int(id)/cols*tile
	dst.DrawImage(a.sheet.SubImage(image.Rect(sx, sy, sx+tile, sy+tile)).(*ebiten.Image), op)
}

func stroke(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	vector.StrokeRect(dst, float32(r.Min.X)+0.5, float32(r.Min.Y)+0.5, float32(r.Dx())-1, float32(r.Dy())-1, 1, c, false)
}

func geo(x, y int, scale float64) *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y))
	return op
}

func fill(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	vector.DrawFilledRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
}

func label(dst *ebiten.Image, x, y int, s string, c color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, face, op)
}

func labelClipped(dst *ebiten.Image, x, y, w int, s string, c color.Color) {
	if n := w / 7; len(s) > n && n > 3 {
		s = s[:n-3] + "..."
	}
	label(dst, x, y, s, c)
}
