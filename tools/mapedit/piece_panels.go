package main

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (p *pieceEditor) drawFileList(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	fill(dst, r, colPanel)
	fill(dst, rect(r.Max.X-1, r.Min.Y, 1, r.Dy()), colLine)
	y := r.Min.Y + padding
	label(dst, r.Min.X+padding, y, "SET PIECES", colTextDim)
	y += lineH + 2
	if v, _ := u.field(dst, rect(r.Min.X+padding, y, r.Dx()-2*padding, lineH+6), "pfilter", p.filter); v != p.filter {
		p.filter = v
	}
	y += lineH + 10
	if br := rect(r.Min.X+padding, y, r.Dx()-2*padding, lineH+4); u.button(dst, br, "New set piece", len(p.tables) > 0) {
		p.popup, p.form = "new", map[string]string{"Level type": "l1", "Name": "new_piece", "Width": "8", "Height": "8"}
	}
	y += lineH + 10

	list := image.Rect(r.Min.X, y, r.Max.X, r.Max.Y)
	fill(dst, list, colPanelAlt)
	var shown []pieceFile
	for _, f := range p.files {
		if p.filter == "" || strings.Contains(f.name, strings.ToLower(p.filter)) {
			shown = append(shown, f)
		}
	}
	u.scrollbar(dst, list, &p.list, len(shown)*lineH)
	off := p.list.clamp(len(shown)*lineH, list.Dy())
	view := dst.SubImage(list).(*ebiten.Image)
	for i, f := range shown {
		row := rect(list.Min.X, list.Min.Y+i*lineH-off, list.Dx()-5, lineH)
		open := p.cur != nil && p.cur.path == f.path
		if open {
			fill(view, row, colAccentDim)
		} else if u.hit(row) && u.hit(list) {
			fill(view, row, colHover)
		}
		name := f.name
		c := colText
		if open && p.dirty {
			name, c = "*"+name, colAccent
		}
		labelClipped(view, row.Min.X+padding, row.Min.Y, row.Dx()-2*padding, name, c)
		if u.hit(row) && u.hit(list) && u.clicked {
			p.open(f, a)
		}
	}
}

// drawRight is the megatile palette above and what the current layer is painting below.
func (p *pieceEditor) drawRight(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	fill(dst, r, colPanel)
	fill(dst, rect(r.Min.X, r.Min.Y, 1, r.Dy()), colLine)
	y := r.Min.Y + padding
	layer := pieceLayers[p.layer]

	label(dst, r.Min.X+padding, y, strings.ToUpper(layer)+" - painting "+strconv.Itoa(p.brush), colAccent)
	y += lineH + 4
	y = labelWrapped(dst, r.Min.X+padding, y, r.Dx()-2*padding, layerHelp[layer], colTextDim) + 4
	if v, done := u.field(dst, rect(r.Min.X+padding, y, 80, lineH+6), "brushval", strconv.Itoa(p.brush)); done {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			p.brush = n
		}
	}
	u.tip(rect(r.Min.X+padding, y, 80, lineH+6), "the value the brush paints; Pick takes it off the map")
	y += lineH + 12

	if layer != "tiles" {
		p.drawLegend(dst, r, y, layer)
		return
	}
	p.drawMegaPalette(a, dst, image.Rect(r.Min.X+1, y, r.Max.X, r.Max.Y))
}

// drawLegend explains a sparse layer, whose ids are the mod's own monster and object numbers.
func (p *pieceEditor) drawLegend(dst *ebiten.Image, r image.Rectangle, y int, layer string) {
	lines := []string{
		"ids are Diablo's own numbers, the",
		"same ones the .DUN files used.",
		"",
		"left-click paints, right-click clears.",
		"0 means nothing here.",
	}
	if layer == "transparency" {
		lines = []string{
			"quarters sharing a number are one",
			"transparency region: the game hides",
			"a whole region at once.",
			"",
			"left-click paints, right-click clears.",
		}
	}
	for i, s := range lines {
		labelClipped(dst, r.Min.X+padding, y+i*lineH, r.Dx()-2*padding, s, colTextDim)
	}
	y += len(lines)*lineH + 10
	if p.cur == nil {
		return
	}
	label(dst, r.Min.X+padding, y, "PLACED", colTextDim)
	y += lineH + 2
	if layer == "transparency" {
		n := 0
		for _, v := range p.cur.trans {
			if v != 0 {
				n++
			}
		}
		labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding, fmt.Sprintf("%d quarters in a region", n), colText)
		return
	}
	counts := map[int]int{}
	for _, v := range p.cur.layers[layer] {
		counts[v]++
	}
	var ids []int
	for id := range counts {
		ids = append(ids, id)
	}
	sortInts(ids)
	for i, id := range ids {
		if y+i*lineH > r.Max.Y-lineH {
			break
		}
		labelClipped(dst, r.Min.X+padding, y+i*lineH, r.Dx()-2*padding, fmt.Sprintf("id %-5d  %d cell(s)", id, counts[id]), colText)
	}
}

func sortInts(v []int) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// drawMegaPalette shows every megatile the level type has, drawn the way the canvas draws them.
func (p *pieceEditor) drawMegaPalette(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	t := p.table()
	if t == nil {
		labelClipped(dst, r.Min.X+padding, r.Min.Y, r.Dx()-2*padding, "open a set piece to see its megatiles", colTextDim)
		return
	}
	fill(dst, r, colPanelAlt)
	const size = 28
	cols := max(1, (r.Dx()-6)/size)
	rows := (t.count() + cols) / cols
	u.scrollbar(dst, r, &p.palette, rows*size)
	off := p.palette.clamp(rows*size, r.Dy())
	view := dst.SubImage(r).(*ebiten.Image)
	for i := 0; i <= t.count(); i++ { // 0 is "no megatile", which is a real choice
		cell := rect(r.Min.X+(i%cols)*size, r.Min.Y+(i/cols)*size-off, size, size)
		if cell.Max.Y < r.Min.Y || cell.Min.Y > r.Max.Y {
			continue
		}
		if i == 0 {
			fill(view, cell.Inset(1), colNoMega)
			label(view, cell.Min.X+8, cell.Min.Y+8, "0", colTextDim)
		} else {
			p.drawMega(a, view, t, p.cur.levelType, i, float64(cell.Min.X+1), float64(cell.Min.Y+1), size-2)
		}
		if i == p.brush {
			stroke(view, cell, color.White)
		}
		if u.hit(cell) && u.hit(r) {
			stroke(view, cell, colAccent)
			u.tooltip = fmt.Sprintf("megatile %d  %s  pieces %v", i, p.megaNames(a, p.cur.levelType, i), t.quarters(i))
			if u.clicked {
				p.brush = i
			}
		}
	}
	_ = vector.StrokeRect
}

func (p *pieceEditor) drawPopup(a *app, dst *ebiten.Image, body image.Rectangle) {
	u := &a.u
	fields := []string{"Width", "Height"}
	title, confirm := "Resize set piece", "Resize"
	if p.popup == "new" {
		fields = []string{"Level type", "Name", "Width", "Height"}
		title, confirm = "New set piece", "Create"
	}
	w, h := 320, 64+len(fields)*(lineH+18)
	r := rect(body.Min.X+(body.Dx()-w)/2, body.Min.Y+(body.Dy()-h)/3, w, h)
	u.popupOpen, u.blocked = true, r
	fill(dst, body, mask)
	fill(dst, r, colPanel)
	stroke(dst, r, colAccent)
	y := r.Min.Y + padding + 2
	label(dst, r.Min.X+padding, y, title, colAccent)
	y += lineH + 6
	for _, f := range fields {
		labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding, f, colTextDim)
		if v, _ := u.field(dst, rect(r.Min.X+padding, y+lineH, r.Dx()-2*padding, lineH+6), "ppopup."+f, p.form[f]); v != p.form[f] {
			p.form[f] = v
		}
		y += lineH + 18
	}
	ok := rect(r.Max.X-padding-90, r.Max.Y-padding-lineH-6, 90, lineH+6)
	cancel := rect(ok.Min.X-96, ok.Min.Y, 90, lineH+6)
	doIt := u.button(dst, ok, confirm, true)
	if u.button(dst, cancel, "Cancel", true) || (!u.typing() && u.key(ebiten.KeyEscape)) {
		p.popup, u.focus = "", ""
		return
	}
	if doIt || (u.focus == "" && u.key(ebiten.KeyEnter)) {
		p.applyPopup(a)
	}
}

func (p *pieceEditor) applyPopup(a *app) {
	w, _ := strconv.Atoi(strings.TrimSpace(p.form["Width"]))
	h, _ := strconv.Atoi(strings.TrimSpace(p.form["Height"]))
	if w <= 0 || h <= 0 {
		a.fail("a set piece needs a positive width and height, in megatiles")
		return
	}
	switch p.popup {
	case "resize":
		p.snapshot()
		p.cur.resize(w, h)
		p.dirty = true
		a.say("resized to %dx%d megatiles", w, h)
	case "new":
		lt := strings.TrimSpace(p.form["Level type"])
		name := strings.TrimSpace(p.form["Name"])
		if p.tables[lt] == nil {
			a.fail("no level type %q: this mod has %s", lt, strings.Join(tableNames(p), ", "))
			return
		}
		if name == "" || strings.ContainsAny(name, `/\: `) {
			a.fail("a set piece name is a file name: no spaces or path characters")
			return
		}
		path := p.levels + "/" + lt + "/" + name + ".txt"
		if fileExists(path) {
			a.fail("%s already exists", path)
			return
		}
		sp := newSetPiece(path, lt, w, h)
		if err := sp.save(); err != nil {
			a.fail("%v", err)
			return
		}
		p.scan()
		p.cur, p.undo, p.redo, p.dirty = sp, nil, nil, false
		a.say("made %s - levels.rec has to name it before the game builds it", path)
	}
	p.popup, a.u.focus = "", ""
}

func tableNames(p *pieceEditor) []string {
	var out []string
	for k := range p.tables {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
