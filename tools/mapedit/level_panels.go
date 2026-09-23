package main

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"rpg/src/hhm"

	"github.com/hajimehoshi/ebiten/v2"
)

// drawLevelList is the left panel: which level is open, and the file operations on it.
func (m *mapEditor) drawLevelList(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	fill(dst, r, colPanel)
	fill(dst, rect(r.Max.X-1, r.Min.Y, 1, r.Dy()), colLine)
	y := r.Min.Y + padding

	label(dst, r.Min.X+padding, y, "LEVELS", colTextDim)
	y += lineH + 2
	if v, done := u.field(dst, rect(r.Min.X+padding, y, r.Dx()-2*padding, lineH+6), "filter", m.filter); done || v != m.filter {
		m.filter = v
	}
	u.tip(rect(r.Min.X+padding, y, r.Dx()-2*padding, lineH+6), "type to narrow the list")
	y += lineH + 10

	bw := (r.Dx() - 2*padding - 4) / 3
	for i, b := range []string{"New", "Copy", "Del"} {
		br := rect(r.Min.X+padding+i*(bw+2), y, bw, lineH+4)
		if u.button(dst, br, b, m.cur != nil || b == "New") {
			switch b {
			case "New":
				m.openPopup("new", map[string]string{"Name": "New_level", "Width": "24", "Height": "16"})
			case "Copy":
				m.openPopup("copy", map[string]string{"Name": m.cur.Name + "2"})
			case "Del":
				m.openPopup("delete", map[string]string{})
			}
		}
	}
	y += lineH + 8
	for i, b := range []string{"Rename", "Resize"} {
		br := rect(r.Min.X+padding+i*(bw*3/2+2), y, bw*3/2-2, lineH+4)
		if u.button(dst, br, b, m.cur != nil) {
			if b == "Rename" {
				m.openPopup("rename", map[string]string{"Name": m.cur.Name})
			} else {
				m.openPopup("resize", map[string]string{"Width": strconv.Itoa(m.cur.Width), "Height": strconv.Itoa(m.cur.Height)})
			}
		}
	}
	y += lineH + 10

	list := image.Rect(r.Min.X, y, r.Max.X, r.Max.Y)
	fill(dst, list, colPanelAlt)
	var shown []*hhm.Level
	for _, l := range m.proj.Levels {
		if m.filter == "" || strings.Contains(strings.ToLower(l.Name), strings.ToLower(m.filter)) {
			shown = append(shown, l)
		}
	}
	u.scrollbar(dst, list, &m.levels, len(shown)*lineH)
	off := m.levels.clamp(len(shown)*lineH, list.Dy())
	view := dst.SubImage(list).(*ebiten.Image)
	for i, l := range shown {
		row := rect(list.Min.X, list.Min.Y+i*lineH-off, list.Dx()-5, lineH)
		if row.Max.Y < list.Min.Y || row.Min.Y > list.Max.Y {
			continue
		}
		if l == m.cur {
			fill(view, row, colAccentDim)
		} else if u.hit(row) && u.hit(list) {
			fill(view, row, colHover)
		}
		name := l.Name
		c := colText
		if m.dirty[l.Name] {
			name, c = "*"+name, colAccent
		}
		labelClipped(view, row.Min.X+padding, row.Min.Y, row.Dx()-2*padding, name, c)
		if u.hit(row) && u.hit(list) && u.clicked {
			m.open(l)
		}
	}
}

// drawRight is the palette above and the inspector below.
func (m *mapEditor) drawRight(a *app, dst *ebiten.Image, r image.Rectangle) {
	fill(dst, r, colPanel)
	fill(dst, rect(r.Min.X, r.Min.Y, 1, r.Dy()), colLine)
	split := r.Min.Y + r.Dy()*55/100
	top := image.Rect(r.Min.X+1, r.Min.Y, r.Max.X, split)
	bottom := image.Rect(r.Min.X+1, split, r.Max.X, r.Max.Y)
	fill(dst, rect(r.Min.X, split-1, r.Dx(), 1), colLine)
	if m.tool == toolEntity || m.tool == toolSelect {
		m.drawEntityPalette(a, dst, top)
	} else {
		m.drawTilePalette(a, dst, top)
	}
	m.drawInspector(a, dst, bottom)
}

func (m *mapEditor) drawTilePalette(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	y := r.Min.Y + padding
	label(dst, r.Min.X+padding, y, fmt.Sprintf("TILES - brush %s", tileLabel(m.brush)), colTextDim)
	y += lineH + 2

	// the markers on the brush tile, which is what the game reads off a drawn tile; a tile that stands for a
	// tiles.rec record takes them from the record's tags instead
	if rec, ok := m.proj.Tileset.Records[m.brush]; ok {
		label(dst, r.Min.X+padding, y, rec+": "+m.drawn(m.brush).Name, colText)
		label(dst, r.Min.X+padding, y+lineH, "tags: "+strings.Join(m.drawn(m.brush).Flags.Names(), ", "), colTextDim)
		label(dst, r.Min.X+padding, y+2*lineH, "edit them in data/icons/tiles.rec", colTextDim)
	} else {
		f := m.proj.Tileset.FlagsFor(m.brush)
		was := f
		cw := (r.Dx() - 2*padding) / 2
		for i, b := range []*bool{&f.BlocksMovement, &f.BlocksView, &f.Water, &f.Damaging, &f.BlocksMissiles, &f.HoldsTrap} {
			cr := rect(r.Min.X+padding+(i%2)*cw, y+(i/2)*lineH, cw, lineH)
			*b = u.checkbox(dst, cr, flagShort[i], *b)
			u.tip(cr, "the brush tile "+flagHelp[i])
		}
		if f != was && m.brush >= 0 {
			m.proj.Tileset.Flags[m.brush] = f
			m.touch() // the tileset is written with the levels
			a.say("tile %d: %v", m.brush, f)
		}
	}
	y += 3*lineH + 6

	grid := image.Rect(r.Min.X, y, r.Max.X, r.Max.Y)
	fill(dst, grid, colPanelAlt)
	ts := m.proj.Tileset
	size := 20 // big enough to tell one 8x8 tile from another
	cols := max(1, (grid.Dx()-6)/size)
	rows := (ts.Count() + cols - 1) / cols
	u.scrollbar(dst, grid, &m.palette, rows*size)
	off := m.palette.clamp(rows*size, grid.Dy())
	view := dst.SubImage(grid).(*ebiten.Image)
	for i := 0; i < ts.Count(); i++ {
		cell := rect(grid.Min.X+(i%cols)*size, grid.Min.Y+(i/cols)*size-off, size, size)
		if cell.Max.Y < grid.Min.Y || cell.Min.Y > grid.Max.Y {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(size)/float64(ts.GridSize), float64(size)/float64(ts.GridSize))
		op.GeoM.Translate(float64(cell.Min.X), float64(cell.Min.Y))
		view.DrawImage(m.tile[i], op)
		if m.showFlags {
			if c := flagTint(m.drawn(i).Flags); c.A > 0 {
				fill(view, cell, c)
			}
		}
		if i == m.brush {
			stroke(view, cell.Inset(-1), color.White)
		}
		if u.hit(cell) && u.hit(grid) {
			stroke(view, cell, colAccent)
			u.tooltip = fmt.Sprintf("tile %d  %s", i, strings.Join(m.drawn(i).Flags.Names(), ", "))
			if rec, ok := ts.Records[i]; ok {
				u.tooltip = fmt.Sprintf("tile %d  %s (%s)  %s", i, m.drawn(i).Name, rec, strings.Join(m.drawn(i).Flags.Names(), ", "))
			}
			if u.clicked {
				m.brush = i
				if m.tool == toolPick {
					m.tool = toolBrush
				}
			}
		}
	}
}

var flagShort = [6]string{"blocks move", "blocks view", "water", "damaging", "blocks missiles", "holds trap"}
var flagHelp = [6]string{"stops movement", "stops sight", "counts as water", "hurts who stands on it",
	"stops arrows and spells", "is a wall a trap may be set into"}

func (m *mapEditor) drawEntityPalette(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	y := r.Min.Y + padding
	label(dst, r.Min.X+padding, y, "ENTITIES", colTextDim)
	y += lineH + 2
	for i, k := range hhm.EntityKinds {
		row := rect(r.Min.X+padding, y+i*(lineH+4), r.Dx()-2*padding, lineH+2)
		if u.toggle(dst, row, k.Name+"   ("+k.Layer+")", i == m.kind) {
			m.kind = i
			m.tool = toolEntity
		}
		u.tip(row, k.Doc)
		sw := rect(row.Max.X-12, row.Min.Y+3, 8, 8)
		fill(dst, sw, kindColor(k.Name))
	}
	y += len(hhm.EntityKinds)*(lineH+4) + 8
	labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding, "click the map to place,", colTextDim)
	labelClipped(dst, r.Min.X+padding, y+lineH, r.Dx()-2*padding, "right-click to remove", colTextDim)
}

// drawInspector edits whatever is selected: an entity's fields, or the level's own.
func (m *mapEditor) drawInspector(a *app, dst *ebiten.Image, r image.Rectangle) {
	u := &a.u
	if m.cur == nil {
		return
	}
	y := r.Min.Y + padding
	if m.sel != nil {
		kind, _ := hhm.KindByName(m.sel.Kind)
		label(dst, r.Min.X+padding, y, strings.ToUpper(m.sel.Kind), colAccent)
		if br := rect(r.Max.X-58, y-2, 50, lineH+2); u.button(dst, br, "Delete", true) {
			m.snapshot()
			m.cur.Remove(m.sel)
			m.sel = nil
			m.touch()
			return
		}
		y += lineH + 2
		y = labelWrapped(dst, r.Min.X+padding, y, r.Dx()-2*padding, kind.Doc, colTextDim) + 2
		labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding, fmt.Sprintf("at %d, %d   on the %s layer", m.sel.X, m.sel.Y, kind.Layer), colTextDim)
		y += lineH + 6
		for _, f := range kind.Fields {
			y = m.drawField(a, dst, r, y, f, m.sel.F[f.Name], func(v string) {
				m.snapshot()
				m.sel.F[f.Name] = v
				m.touch()
			})
		}
		return
	}

	label(dst, r.Min.X+padding, y, "LEVEL", colAccent)
	y += lineH + 2
	labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding,
		fmt.Sprintf("%s   %dx%d   %d entities", m.cur.Name, m.cur.Width, m.cur.Height, len(m.cur.Entities)), colTextDim)
	y += lineH + 6
	for _, f := range hhm.LevelFields {
		y = m.drawField(a, dst, r, y, f, hhm.Prop(m.cur.Props, f.Name), func(v string) {
			m.snapshot()
			hhm.SetProp(&m.cur.Props, f.Name, v)
			m.touch()
		})
	}
}

// drawField renders one schema field as the widget its type calls for, and returns the next y.
func (m *mapEditor) drawField(a *app, dst *ebiten.Image, r image.Rectangle, y int, f hhm.FieldDef, value string, set func(string)) int {
	u := &a.u
	w := r.Dx() - 2*padding
	x := r.Min.X + padding
	switch f.Type {
	case hhm.FBool:
		cr := rect(x, y, w, lineH)
		if now := u.checkbox(dst, cr, f.Name, value == "true"); now != (value == "true") {
			set(strconv.FormatBool(now))
		}
		u.tip(cr, f.Doc)
		return y + lineH + 4
	case hhm.FEnum:
		labelClipped(dst, x, y, w, f.Name, colTextDim)
		y += lineH
		per := 2
		bw := w / per
		for i, v := range f.Values {
			br := rect(x+(i%per)*bw, y+(i/per)*(lineH+2), bw-2, lineH)
			if u.toggle(dst, br, v, v == value) {
				set(v)
			}
		}
		return y + ((len(f.Values)+per-1)/per)*(lineH+2) + 4
	case hhm.FTile:
		labelClipped(dst, x, y, w, f.Name+"  (click to take the brush)", colTextDim)
		y += lineH
		id, _ := strconv.Atoi(value)
		br := rect(x, y, lineH+8, lineH+8)
		fill(dst, br, colPanelAlt)
		if id >= 0 && id < len(m.tile) {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(float64(lineH+8)/float64(m.proj.Tileset.GridSize), float64(lineH+8)/float64(m.proj.Tileset.GridSize))
			op.GeoM.Translate(float64(br.Min.X), float64(br.Min.Y))
			dst.DrawImage(m.tile[id], op)
		}
		stroke(dst, br, colLine)
		if u.hit(br) && u.clicked {
			set(strconv.Itoa(m.brush))
		}
		if v, done := u.field(dst, rect(br.Max.X+4, y+4, w-br.Dx()-4, lineH+4), f.Name, value); done && v != value {
			set(v)
		}
		return y + lineH + 14
	default:
		labelClipped(dst, x, y, w, f.Name, colTextDim)
		y += lineH
		fr := rect(x, y, w, lineH+6)
		if v, done := u.field(dst, fr, f.Name, value); done && v != value {
			set(v)
		}
		u.tip(fr, f.Doc+"   (\\n starts a new line)")
		return y + lineH + 10
	}
}
