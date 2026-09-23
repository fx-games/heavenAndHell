package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"rpg/src/hhm"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The tools, in the order their number keys run.
const (
	toolBrush = iota
	toolRect
	toolFill
	toolPick
	toolErase
	toolEntity
	toolSelect
	numTools
)

var toolNames = [numTools]string{"Brush", "Rect", "Fill", "Pick", "Erase", "Entity", "Select"}
var toolHelp = [numTools]string{
	"Brush 1 - drag to paint the chosen tile",
	"Rect 2 - drag a rectangle to fill it with the chosen tile; hold Shift to outline it",
	"Fill 3 - click to flood the connected area of like tiles",
	"Pick 4 - click a tile to make it the brush",
	"Erase 5 - drag to clear tiles back to empty",
	"Entity 6 - click to place the chosen entity, right-click to remove one",
	"Select 7 - click an entity to edit its fields, drag to move it, Delete removes it",
}

type mapEditor struct {
	dir   string // the mod directory
	mapsD string // its maps directory
	proj  *hhm.Project
	sheet *ebiten.Image
	tile  []*ebiten.Image // one sub-image per tile index

	cur    *hhm.Level
	tool   int
	brush  int
	kind   int // index into hhm.EntityKinds
	sel    *hhm.Entity
	drag   *hhm.Entity // the entity being moved
	rectA  image.Point
	rectOn bool

	showGrid, showFlags, showNames bool
	cam                            camera
	panX, panY                     int
	panning                        bool
	dirty                          map[string]bool
	undo, redo                     []*hhm.Level

	levels, palette, inspect scroller
	filter                   string
	popup                    string // "" or the name of the open dialog
	form                     map[string]string
}

type camera struct {
	x, y float64
	zoom float64
}

func newMapEditor(dir string) (*mapEditor, error) {
	m := &mapEditor{dir: dir, mapsD: filepath.Join(dir, "maps"), showGrid: true, showNames: true,
		cam: camera{zoom: 3}, dirty: map[string]bool{}, form: map[string]string{}}
	if err := m.reload(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *mapEditor) reload() error {
	p, err := hhm.Load(os.DirFS(m.dir), "maps")
	if err != nil {
		return err
	}
	sheet, _, err := ebitenutil.NewImageFromFile(filepath.Join(m.dir, filepath.FromSlash(p.Tileset.Image)))
	if err != nil {
		return fmt.Errorf("tileset image: %w", err)
	}
	m.proj, m.sheet = p, sheet
	m.tile = m.tile[:0]
	g, cols := p.Tileset.GridSize, p.Tileset.Columns()
	for i := 0; i < p.Tileset.Count(); i++ {
		r := image.Rect((i%cols)*g, (i/cols)*g, (i%cols)*g+g, (i/cols)*g+g)
		m.tile = append(m.tile, sheet.SubImage(r).(*ebiten.Image))
	}
	if len(p.Levels) > 0 {
		m.open(p.Levels[0])
	}
	return nil
}

func (m *mapEditor) open(l *hhm.Level) {
	m.cur, m.sel, m.undo, m.redo = l, nil, nil, nil
	m.cam.x, m.cam.y = -16, -16
}

// cellPx is how many screen pixels one map cell covers.
func (m *mapEditor) cellPx() float64 { return float64(m.proj.Tileset.GridSize) * m.cam.zoom }

func (m *mapEditor) topBar(a *app, dst *ebiten.Image, x, w int) {
	u := &a.u
	for i := 0; i < numTools; i++ {
		name := fmt.Sprintf("%d %s", i+1, toolNames[i])
		bw := textWidth(name) + 2*padding + 4
		r := rect(x, 3, bw, topH-7)
		if u.toggle(dst, r, name, m.tool == i) {
			m.tool = i
		}
		u.tip(r, toolHelp[i])
		x += bw + 2
	}
	x += 8
	for _, b := range []struct {
		name, tip string
		on        *bool
	}{
		{"Grid", "G - show the cell grid", &m.showGrid},
		{"Flags", "F - tint tiles by what they block", &m.showFlags},
		{"Names", "N - label the entities", &m.showNames},
	} {
		bw := textWidth(b.name) + 2*padding + 4
		r := rect(x, 3, bw, topH-7)
		if u.toggle(dst, r, b.name, *b.on) {
			*b.on = !*b.on
		}
		u.tip(r, b.tip)
		x += bw + 2
	}
	x += 8
	if r := rect(x, 3, 66, topH-7); true {
		n := len(m.dirty)
		s := "Save"
		if n > 0 {
			s = fmt.Sprintf("Save (%d)", n)
		}
		if u.button(dst, r, s, n > 0) {
			m.save(a)
		}
		u.tip(r, "Ctrl+S - write every changed level back to "+m.mapsD)
		x += 70
	}
	if r := rect(x, 3, 56, topH-7); u.button(dst, r, "Reload", true) {
		if err := m.reload(); err != nil {
			a.fail("%v", err)
		} else {
			m.dirty = map[string]bool{}
			a.say("reloaded from disk")
		}
	}
}

func (m *mapEditor) draw(a *app, dst *ebiten.Image, body image.Rectangle) {
	u := &a.u
	if m.proj == nil {
		return
	}
	left := image.Rect(body.Min.X, body.Min.Y, body.Min.X+leftW, body.Max.Y)
	right := image.Rect(body.Max.X-rightW, body.Min.Y, body.Max.X, body.Max.Y)
	canvas := image.Rect(left.Max.X, body.Min.Y, right.Min.X, body.Max.Y)

	m.keys(a)
	m.drawCanvas(a, dst, canvas)
	m.drawLevelList(a, dst, left)
	m.drawRight(a, dst, right)
	if m.popup != "" {
		m.drawPopup(a, dst, body)
	} else {
		u.popupOpen = false
	}
	_ = u
}

func (m *mapEditor) keys(a *app) {
	u := &a.u
	if u.typing() || m.popup != "" {
		return
	}
	for i := 0; i < numTools; i++ {
		if u.key(ebiten.Key(int(ebiten.Key1) + i)) {
			m.tool = i
		}
	}
	if u.key(ebiten.KeyG) {
		m.showGrid = !m.showGrid
	}
	if u.key(ebiten.KeyF) {
		m.showFlags = !m.showFlags
	}
	if u.key(ebiten.KeyN) {
		m.showNames = !m.showNames
	}
	if ctrl() && u.key(ebiten.KeyS) {
		m.save(a)
	}
	if ctrl() && u.key(ebiten.KeyZ) {
		if shiftHeld() {
			m.doRedo(a)
		} else {
			m.doUndo(a)
		}
	}
	if m.sel != nil && (u.key(ebiten.KeyDelete) || u.key(ebiten.KeyBackspace)) {
		m.snapshot()
		m.cur.Remove(m.sel)
		m.sel = nil
		m.touch()
	}
	speed := 12.0
	if shiftHeld() {
		speed = 48
	}
	for k, d := range map[ebiten.Key][2]float64{
		ebiten.KeyArrowLeft: {-1, 0}, ebiten.KeyArrowRight: {1, 0},
		ebiten.KeyArrowUp: {0, -1}, ebiten.KeyArrowDown: {0, 1},
	} {
		if ebiten.IsKeyPressed(k) {
			m.cam.x += d[0] * speed
			m.cam.y += d[1] * speed
		}
	}
}

// touch marks the open level as changed.
func (m *mapEditor) touch() {
	if m.cur != nil {
		m.dirty[m.cur.Name] = true
	}
}

func cloneLevel(l *hhm.Level) *hhm.Level {
	c := *l
	c.Tiles = append([]int(nil), l.Tiles...)
	c.Entities = nil
	for _, e := range l.Entities {
		c.Entities = append(c.Entities, e.Clone())
	}
	return &c
}

// snapshot records the level as it is, so the next change can be undone.
func (m *mapEditor) snapshot() {
	if m.cur == nil {
		return
	}
	m.undo = append(m.undo, cloneLevel(m.cur))
	if len(m.undo) > 64 {
		m.undo = m.undo[1:]
	}
	m.redo = nil
}

func (m *mapEditor) doUndo(a *app) {
	if len(m.undo) == 0 {
		a.say("nothing to undo")
		return
	}
	m.redo = append(m.redo, cloneLevel(m.cur))
	m.restore(m.undo[len(m.undo)-1])
	m.undo = m.undo[:len(m.undo)-1]
	a.say("undone")
}

func (m *mapEditor) doRedo(a *app) {
	if len(m.redo) == 0 {
		a.say("nothing to redo")
		return
	}
	m.undo = append(m.undo, cloneLevel(m.cur))
	m.restore(m.redo[len(m.redo)-1])
	m.redo = m.redo[:len(m.redo)-1]
	a.say("redone")
}

func (m *mapEditor) restore(to *hhm.Level) {
	*m.cur = *to
	m.cur.Tiles = append([]int(nil), to.Tiles...)
	m.cur.Entities = nil
	for _, e := range to.Entities {
		m.cur.Entities = append(m.cur.Entities, e.Clone())
	}
	m.sel = nil
	m.touch()
}

func (m *mapEditor) save(a *app) {
	if len(m.dirty) == 0 {
		a.say("nothing to save")
		return
	}
	if err := os.WriteFile(filepath.Join(m.mapsD, hhm.TilesetFile), hhm.WriteTileset(m.proj.Tileset), 0o644); err != nil {
		a.fail("%v", err)
		return
	}
	n := 0
	for name := range m.dirty {
		l := m.proj.Level(name)
		if l == nil {
			continue // a level that was renamed or deleted: its old file is gone already
		}
		if err := hhm.SaveLevel(m.mapsD, l); err != nil {
			a.fail("%v", err)
			return
		}
		n++
	}
	m.dirty = map[string]bool{}
	a.say("saved %d level(s) to %s", n, m.mapsD)
}
