package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The layers a set piece has. Tiles is one value per megatile; the rest are at twice that resolution, the way
// Diablo's .DUN files are.
var pieceLayers = []string{"tiles", "transparency", "items", "monsters", "objects"}

var layerHelp = map[string]string{
	"tiles":        "the megatile the generator stamps here; 0 leaves the generator's own",
	"transparency": "the transparency region this quarter belongs to; 0 for none",
	"items":        "the item to drop on this quarter",
	"monsters":     "the monster to put on this quarter",
	"objects":      "the object to put on this quarter",
}

type pieceEditor struct {
	dir    string // the mod directory
	levels string // its data/levels directory

	files  []pieceFile
	tables map[string]*megaTable

	cur        *setPiece
	layer      int
	brush      int // the megatile or id being painted
	tool       int
	cam        camera
	showIDs    bool
	dirty      bool
	undo, redo []*setPiece

	list, palette scroller
	filter        string
	panX, panY    int
	rectA         image.Point
	rectOn        bool
	panning       bool
	popup         string
	form          map[string]string
	err           string
}

type pieceFile struct {
	path, levelType, name string
}

func newPieceEditor(dir string) *pieceEditor {
	p := &pieceEditor{dir: dir, levels: filepath.Join(dir, "data", "levels"),
		tables: map[string]*megaTable{}, cam: camera{zoom: 2}, showIDs: true, form: map[string]string{}, brush: 1}
	p.scan()
	return p
}

// scan finds every set piece the mod ships, by level type.
func (p *pieceEditor) scan() {
	p.files = nil
	for _, lt := range []string{"l1", "l2", "l3", "l4"} {
		d := filepath.Join(p.levels, lt)
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		if t, err := readMegaTable(d); err == nil {
			p.tables[lt] = t
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
				p.files = append(p.files, pieceFile{filepath.Join(d, e.Name()), lt, lt + "/" + strings.TrimSuffix(e.Name(), ".txt")})
			}
		}
	}
	sort.Slice(p.files, func(i, j int) bool { return p.files[i].name < p.files[j].name })
}

func (p *pieceEditor) table() *megaTable {
	if p.cur == nil {
		return nil
	}
	return p.tables[p.cur.levelType]
}

// cellPx is how many screen pixels one megatile covers.
func (p *pieceEditor) cellPx() float64 { return 16 * p.cam.zoom }

func (p *pieceEditor) topBar(a *app, dst *ebiten.Image, x, w int) {
	u := &a.u
	for i, name := range []string{"Brush", "Rect", "Fill", "Pick", "Erase"} {
		bw := textWidth(fmt.Sprintf("%d %s", i+1, name)) + 2*padding + 4
		r := rect(x, 3, bw, topH-7)
		if u.toggle(dst, r, fmt.Sprintf("%d %s", i+1, name), p.tool == i) {
			p.tool = i
		}
		x += bw + 2
	}
	x += 10
	for i, name := range pieceLayers {
		bw := textWidth(name) + 2*padding + 4
		r := rect(x, 3, bw, topH-7)
		if u.toggle(dst, r, name, p.layer == i) {
			p.layer = i
		}
		u.tip(r, layerHelp[name])
		x += bw + 2
	}
	x += 10
	r := rect(x, 3, 44, topH-7)
	if u.toggle(dst, r, "IDs", p.showIDs) {
		p.showIDs = !p.showIDs
	}
	u.tip(r, "I - write the numbers on the cells")
	x += 48
	if r := rect(x, 3, 56, topH-7); u.button(dst, r, "Save", p.dirty) {
		p.save(a)
	}
	x += 60
	if r := rect(x, 3, 66, topH-7); u.button(dst, r, "New size", p.cur != nil) {
		p.popup, p.form = "resize", map[string]string{"Width": strconv.Itoa(p.cur.w), "Height": strconv.Itoa(p.cur.h)}
	}
}

func (p *pieceEditor) draw(a *app, dst *ebiten.Image, body image.Rectangle) {
	u := &a.u
	left := image.Rect(body.Min.X, body.Min.Y, body.Min.X+leftW, body.Max.Y)
	right := image.Rect(body.Max.X-rightW, body.Min.Y, body.Max.X, body.Max.Y)
	canvas := image.Rect(left.Max.X, body.Min.Y, right.Min.X, body.Max.Y)

	if p.cur == nil && len(p.files) > 0 {
		p.open(p.files[0], a) // a mode that opens onto nothing is a mode nobody uses
	}
	p.keys(a)
	p.drawCanvas(a, dst, canvas)
	p.drawFileList(a, dst, left)
	p.drawRight(a, dst, right)
	if p.popup != "" {
		p.drawPopup(a, dst, body)
	} else {
		u.popupOpen = false
	}
}

func (p *pieceEditor) keys(a *app) {
	u := &a.u
	if u.typing() || p.popup != "" {
		return
	}
	for i := 0; i < 5; i++ {
		if u.key(ebiten.Key(int(ebiten.Key1) + i)) {
			p.tool = i
		}
	}
	if u.key(ebiten.KeyI) {
		p.showIDs = !p.showIDs
	}
	if ctrl() && u.key(ebiten.KeyS) {
		p.save(a)
	}
	if ctrl() && u.key(ebiten.KeyZ) {
		if shiftHeld() {
			p.swap(&p.redo, &p.undo, a, "redone")
		} else {
			p.swap(&p.undo, &p.redo, a, "undone")
		}
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
			p.cam.x += d[0] * speed
			p.cam.y += d[1] * speed
		}
	}
}

func (p *pieceEditor) snapshot() {
	if p.cur == nil {
		return
	}
	p.undo = append(p.undo, p.cur.clone())
	if len(p.undo) > 64 {
		p.undo = p.undo[1:]
	}
	p.redo = nil
}

func (p *pieceEditor) swap(from, to *[]*setPiece, a *app, what string) {
	if len(*from) == 0 {
		a.say("nothing to %s", strings.TrimSuffix(what, "ne"))
		return
	}
	*to = append(*to, p.cur.clone())
	*p.cur = *(*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	p.dirty = true
	a.say("%s", what)
}

func (p *pieceEditor) open(f pieceFile, a *app) {
	sp, err := readSetPiece(f.path, f.levelType)
	if err != nil {
		a.fail("%v", err)
		return
	}
	p.cur, p.undo, p.redo, p.dirty = sp, nil, nil, false
	p.cam.x, p.cam.y = -16, -16
	a.say("%s - %dx%d megatiles, %s tables", f.name, sp.w, sp.h, f.levelType)
}

func (p *pieceEditor) save(a *app) {
	if p.cur == nil || !p.dirty {
		a.say("nothing to save")
		return
	}
	if err := p.cur.save(); err != nil {
		a.fail("%v", err)
		return
	}
	p.dirty = false
	a.say("saved %s", p.cur.path)
}
