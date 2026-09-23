package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rpg/src/hhm"

	"github.com/hajimehoshi/ebiten/v2"
)

// popupSpec says what each dialog asks for and what its confirm button says.
var popupSpec = map[string]struct {
	title, confirm string
	fields         []string
}{
	"new":    {"New level", "Create", []string{"Name", "Width", "Height"}},
	"copy":   {"Copy level", "Copy", []string{"Name"}},
	"rename": {"Rename level", "Rename", []string{"Name"}},
	"resize": {"Resize level", "Resize", []string{"Width", "Height"}},
	"delete": {"Delete level", "Delete", nil},
}

func (m *mapEditor) openPopup(name string, form map[string]string) {
	m.popup, m.form = name, form
}

func (m *mapEditor) drawPopup(a *app, dst *ebiten.Image, body image.Rectangle) {
	u := &a.u
	spec := popupSpec[m.popup]
	w, h := 300, 64+len(spec.fields)*(lineH+18)
	r := rect(body.Min.X+(body.Dx()-w)/2, body.Min.Y+(body.Dy()-h)/3, w, h)
	u.popupOpen, u.blocked = true, r

	fill(dst, body, mask)
	fill(dst, r, colPanel)
	stroke(dst, r, colAccent)
	y := r.Min.Y + padding + 2
	label(dst, r.Min.X+padding, y, spec.title, colAccent)
	y += lineH + 6
	if m.popup == "delete" {
		labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding, "delete "+m.cur.Name+"?", colText)
		labelClipped(dst, r.Min.X+padding, y+lineH, r.Dx()-2*padding, "its file goes too, right away", colTextDim)
	}
	for _, f := range spec.fields {
		labelClipped(dst, r.Min.X+padding, y, r.Dx()-2*padding, f, colTextDim)
		if v, _ := u.field(dst, rect(r.Min.X+padding, y+lineH, r.Dx()-2*padding, lineH+6), "popup."+f, m.form[f]); v != m.form[f] {
			m.form[f] = v
		}
		y += lineH + 18
	}
	ok := rect(r.Max.X-padding-90, r.Max.Y-padding-lineH-6, 90, lineH+6)
	cancel := rect(ok.Min.X-96, ok.Min.Y, 90, lineH+6)
	confirm := u.button(dst, ok, spec.confirm, true)
	if u.button(dst, cancel, "Cancel", true) || (!u.typing() && u.key(ebiten.KeyEscape)) {
		m.popup, u.focus = "", ""
		return
	}
	if confirm || (u.focus == "" && u.key(ebiten.KeyEnter)) {
		m.applyPopup(a)
	}
}

var mask = colorRGBA(0, 0, 0, 150)

func (m *mapEditor) applyPopup(a *app) {
	name := strings.TrimSpace(m.form["Name"])
	w, _ := strconv.Atoi(m.form["Width"])
	h, _ := strconv.Atoi(m.form["Height"])
	switch m.popup {
	case "new", "copy":
		if err := m.checkName(name); err != nil {
			a.fail("%v", err)
			return
		}
		var l *hhm.Level
		if m.popup == "new" {
			if w <= 0 || h <= 0 {
				a.fail("a level needs a positive width and height")
				return
			}
			l = hhm.NewLevel(name, w, h)
		} else {
			l = cloneLevel(m.cur)
			l.Name, l.Props.DisplayName = name, m.cur.Props.DisplayName
		}
		m.proj.Add(l)
		m.open(l)
		m.dirty[name] = true
		a.say("made %s", name)
	case "rename":
		if name == m.cur.Name {
			break
		}
		if err := m.checkName(name); err != nil {
			a.fail("%v", err)
			return
		}
		old := m.cur.Name
		m.cur.Name = name
		delete(m.dirty, old)
		m.dirty[name] = true
		if err := os.Remove(filepath.Join(m.mapsD, old+hhm.Ext)); err != nil && !os.IsNotExist(err) {
			a.fail("%v", err)
			return
		}
		a.say("renamed %s to %s - levels.rec still names the old one", old, name)
	case "resize":
		if w <= 0 || h <= 0 {
			a.fail("a level needs a positive width and height")
			return
		}
		m.snapshot()
		m.cur.Resize(w, h)
		m.touch()
		a.say("resized to %dx%d", w, h)
	case "delete":
		gone := m.cur.Name
		for i, l := range m.proj.Levels {
			if l == m.cur {
				m.proj.Levels = append(m.proj.Levels[:i], m.proj.Levels[i+1:]...)
				break
			}
		}
		delete(m.dirty, gone)
		if err := os.Remove(filepath.Join(m.mapsD, gone+hhm.Ext)); err != nil && !os.IsNotExist(err) {
			a.fail("%v", err)
			return
		}
		m.cur, m.sel = nil, nil
		if len(m.proj.Levels) > 0 {
			m.open(m.proj.Levels[0])
		}
		a.say("deleted %s", gone)
	}
	m.popup, a.u.focus = "", ""
}

// checkName refuses a level name that would not make a sane file, or that is already taken.
func (m *mapEditor) checkName(name string) error {
	if name == "" {
		return fmt.Errorf("a level needs a name")
	}
	if strings.ContainsAny(name, `/\:*?"<>| `) {
		return fmt.Errorf("a level name is a file name: no spaces or path characters")
	}
	if m.proj.Level(name) != nil {
		return fmt.Errorf("%s already exists", name)
	}
	return nil
}
