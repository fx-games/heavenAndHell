// spawnedit assigns the mod's randomly rolled monsters to its dungeon levels: drag a monster from the right onto
// a level, click it to raise its spawn_cost (shift-click to lower it), right-click it off again. A monster spawns on
// the depths dlvl..max_dlvl (enemies.rec, with random_spawn and spawn_cost); a side level's monster lines live in
// levels.rec. Both mods pick types and counts by D1's rules. A unique goes where its room goes: drag it onto an
// accretion level to require its room (maps/*.hhm) there once it fits, and set its dlvl to that depth; a unique a quest
// spawns on entering a level (quests.rec) moves its trigger, onto any level. A development tool, kept out of the game binary.
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
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
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
	"rpg/src/hhm"
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
	rooms                   map[string]string // unique id -> the room template that places it
	spawns                  map[string]*spawn // unique id -> the quest trigger that spawns it
	questsPath              string

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
		questsPath: filepath.Join(dir, "data", "quests.rec"),
		dragging:   -1, room: map[string]*ebiten.Image{}, colors: map[string]color.RGBA{}, msg: "editing " + dir}
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
		a.enemies = append(a.enemies, &enemy{id: m.ID, name: m.Name, icon: icon, min: m.MinDLvl, max: m.MaxDLvl, avail: b2i(m.RandomSpawn),
			cost: m.SpawnCost, unique: slices.Contains(m.Tags, "unique"), danger: m.Exp})
		if m.Color.A == 0 { // no color field: still visible in the share bar
			m.Color = colDim
		}
		a.colors[m.ID] = m.Color
	}
	mapFiles, err := hhm.Load(os.DirFS(filepath.Dir(filepath.Dir(a.levelsPath))), "maps")
	if err != nil {
		return err
	}
	a.rooms = roomsOf(mapFiles)
	if b, err := os.ReadFile(a.questsPath); err == nil {
		m := recfile.ReadMulti(strings.NewReader(string(b)))
		a.spawns = questSpawns(slices.Concat(m["quest"], m["trigger"]))
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
		l := &level{rec: r}
		l.id, _ = dungeon.Field(r, "map")
		l.name, _ = dungeon.Field(r, "name")
		l.name = strings.ReplaceAll(l.name, `\n`, " - ")
		l.tiles, _ = dungeon.Field(r, "tiles")
		l.depth, _ = dungeon.Int(r, "depth", 0)
		l.generator, _ = dungeon.Field(r, "generator")
		l.prefix, _ = dungeon.Field(r, "template_prefix")
		for _, v := range dungeon.FieldsNamed(r, "require_room") {
			l.require = append(l.require, strings.TrimSpace(v))
		}
		a.levels = append(a.levels, l)
		if !slices.Contains(a.depths, l.depth) {
			a.depths = append(a.depths, l.depth)
		}
	}
	// a room counts once a level requires it: a drawn map (Lucifer's TownHell) holds uniques too, but no level grows it
	maps.DeleteFunc(a.rooms, func(_, room string) bool {
		return !slices.ContainsFunc(a.levels, func(l *level) bool { return slices.Contains(l.require, room) })
	})
	slices.SortStableFunc(a.levels, func(x, y *level) int { return x.depth - y.depth })
	slices.Sort(a.depths)
	// ponytail: experience paid stands in for danger; a real threat score (hp x damage x speed) if it misleads
	slices.SortStableFunc(a.enemies, func(x, y *enemy) int { return cmp.Or(b2i(x.unique)-b2i(y.unique), x.danger-y.danger) })
	return nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// here is who level l may roll: a side level its list, any other everyone whose dlvl..max_dlvl takes in its depth.
func (a *app) here(l *level) []*enemy {
	var out []*enemy
	for _, e := range a.enemies {
		if e.unique && (slices.Contains(l.require, a.rooms[e.id]) || a.spawns[e.id] != nil && a.spawns[e.id].to == l.id) ||
			l.side() && slices.Contains(l.mons, e.id) || !l.side() && !e.unique && e.spawnsAt(l.depth, a.deepest()) {
			out = append(out, e)
		}
	}
	return out
}

// toggle puts e on, or takes it off, depth d.
func (a *app) toggle(e *enemy, d int, on bool) {
	if !on {
		e.add(d, a.deepest())
	} else if err := e.remove(d, a.deepest()); err != nil {
		a.msg = err.Error()
		return
	}
	a.touch("%s on depth %d: %v", e.name, d, !on)
}

// moveUnique moves e to level l, with its room or the quest trigger that spawns it; its dlvl (and so its
// monster level) follows l's depth, and so does every other monster the trigger spawns.
func (a *app) moveUnique(e *enemy, l *level) bool {
	if sp := a.spawns[e.id]; sp != nil {
		sp.to = l.id
		for _, o := range a.enemies {
			if a.spawns[o.id] == sp {
				o.min = l.depth
			}
		}
		a.touch("the trigger spawning %s now fires on %s (with every monster it spawns)", e.name, l.id)
		return true
	}
	room := a.rooms[e.id]
	if !l.grows(room) {
		a.msg = fmt.Sprintf("%s's room %s can't grow on %s (an accretion level with template_prefix %q)", e.name, room, l.id, l.prefix)
		return false
	}
	if err := placeable(l, append(slices.Clone(l.require), room)); err != nil {
		a.msg = fmt.Sprintf("%s's room doesn't fit on %s: %v", e.name, l.id, err)
		return false
	}
	moveRoom(a.levels, room, l)
	e.min = l.depth
	a.touch("%s and its room %s now on %s (placement checked)", e.name, room, l.id)
	return true
}

// uniqueLevel is the level e is on now: the one requiring its room or firing its trigger; nil when neither.
func (a *app) uniqueLevel(e *enemy) *level {
	for _, l := range a.levels {
		if sp := a.spawns[e.id]; sp != nil && sp.to == l.id || slices.Contains(l.require, a.rooms[e.id]) {
			return l
		}
	}
	return nil
}

// moveUniqueTo moves e to the first level of depth d that takes it: a unique's row of the matrix is a radio group.
func (a *app) moveUniqueTo(e *enemy, d int) {
	for _, l := range a.levels {
		if l.depth == d && a.moveUnique(e, l) {
			return
		}
	}
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
			if e.unique {
				a.moveUnique(e, l)
			} else if l.side() {
				l.setOn(e.id, true)
			} else {
				e.add(l.depth, a.deepest())
			}
			if !e.unique {
				a.touch("%s now spawns on %s", e.name, l.id)
			}
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
		if err := save(a.enemiesPath, a.levelsPath, a.questsPath, a.enemies, a.levels, a.spawns); err != nil {
			a.msg = "save failed: " + err.Error()
		} else {
			a.dirty, a.msg = false, "saved enemies.rec, levels.rec and quests.rec"
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
	head += "; click +100 spawn_cost, shift-click -100"
	label(dst, 6, 5, head, colDim)
	label(dst, split+10, 5, "Monsters: drag onto a level; cells: the depths it spawns on", colDim)

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
		// who may spawn here, and what each costs of the budget
		here := a.here(l)
		total := 0
		for _, e := range here {
			total += e.cost
		}
		label(view, x, row.Min.Y+20, fmt.Sprintf("types cost %d of %d", total, defs.SpawnBudget), colDim)
		cx, cy := x, row.Min.Y+38
		for _, e := range here {
			if cx+18 > row.Max.X {
				continue // ponytail: one line of chips, the bar below still counts them all
			}
			a.icon(view, e.icon, cx, cy, 2)
			label(view, cx, cy+16, fmt.Sprint(e.cost/100), colDim)
			tip := fmt.Sprintf("%s: spawn_cost %d (click +100, shift-click -100, right-click remove)", e.name, e.cost)
			a.hots = append(a.hots, hot{r: image.Rect(cx, cy, cx+16, cy+16).Intersect(r), drag: -1, drop: i, tip: tip,
				left: func() {
					if ebiten.IsKeyPressed(ebiten.KeyShift) {
						e.cost = max(0, e.cost-100)
					} else {
						e.cost += 100
					}
					a.touch("%s: spawn_cost %d", e.name, e.cost)
				},
				right: func() {
					if e.unique {
						a.msg = e.name + " goes where its room goes: drag it onto another level"
						return
					}
					if l.side() {
						l.setOn(e.id, false)
					} else if err := e.remove(l.depth, a.deepest()); err != nil {
						a.msg = err.Error()
						return
					}
					a.touch("%s no longer spawns on %s", e.name, l.id)
				}})
			cx += 18
		}
		bx, bw := float64(x), float64(row.Max.X-x-8)/float64(max(total, defs.SpawnBudget)) // the budget's width: what sticks out won't fit
		for _, e := range here {
			w := bw * float64(e.cost)
			vector.DrawFilledRect(view, float32(bx), float32(row.Min.Y+70), float32(w)-1, 8, a.colors[e.id], false)
			bx += w
		}
		stroke(view, image.Rect(x, row.Min.Y+68, x+int(bw*defs.SpawnBudget), row.Min.Y+80), colDim)
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
		nameR := image.Rect(row.Min.X, row.Min.Y, chart-4, row.Max.Y)
		if e.unique { // read only: its room places it
			labelClipped(view, row.Min.X+22, row.Min.Y+3, nameR.Dx()-24, e.name+" (unique)", colDim)
			h := hot{r: nameR.Intersect(body), drag: -1, drop: -1, tip: fmt.Sprintf("%s: unique, placed by its room; level %d", e.id, e.min)}
			if room := a.rooms[e.id]; room != "" {
				h.drag, h.tip = j, fmt.Sprintf("%s: unique in room %s, level %d; drag onto a level to move both", e.id, room, e.min)
			} else if sp := a.spawns[e.id]; sp != nil {
				h.drag, h.tip = j, fmt.Sprintf("%s: unique spawned by a quest on %s; drag onto a level to move its trigger", e.id, sp.to)
			}
			a.hots = append(a.hots, h)
			cur := a.uniqueLevel(e)
			for k, d := range a.depths {
				cell := image.Rect(chart+k*cellW, row.Min.Y+3, chart+k*cellW+cellW-2, row.Max.Y-3)
				on := cur != nil && cur.depth == d
				switch {
				case on:
					fill(view, cell, colOn)
				case h.drag >= 0:
					fill(view, cell, colLine)
				default: // no room or trigger to move: read only
					stroke(view, cell, colLine)
					continue
				}
				a.hots = append(a.hots, hot{r: cell.Intersect(body), drag: -1, drop: -1, tip: fmt.Sprintf("%s on depth %d: click to move it here", e.name, d),
					left: func() {
						if !on {
							a.moveUniqueTo(e, d)
						}
					}})
			}
			continue
		}
		c := colText
		if !slices.ContainsFunc(a.depths, func(d int) bool { return e.spawnsAt(d, a.deepest()) }) {
			c = colDim
		}
		labelClipped(view, row.Min.X+22, row.Min.Y+3, nameR.Dx()-24, e.name, c)
		a.hots = append(a.hots, hot{r: nameR.Intersect(body), drag: j, drop: -1, tip: e.id + ": drag onto a level"})
		for k, d := range a.depths {
			cell := image.Rect(chart+k*cellW, row.Min.Y+3, chart+k*cellW+cellW-2, row.Max.Y-3)
			on := e.spawnsAt(d, a.deepest())
			a.hots = append(a.hots, hot{r: cell.Intersect(body), drag: -1, drop: -1, tip: fmt.Sprintf("%s on depth %d: click or drag to toggle", e.name, d), cell: fmt.Sprint(e.id, "@", d), on: on,
				left: func() { a.toggle(e, d, on) }})
			if on {
				fill(view, cell, colOn)
			} else {
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
