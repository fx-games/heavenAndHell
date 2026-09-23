package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"rpg/src/hhm"
)

// handleMouse runs the active tool over the cell the pointer is on.
func (m *mapEditor) handleMouse(a *app, c image.Rectangle, x, y int) {
	u := &a.u
	if u.middle || (ebiten.IsKeyPressed(ebiten.KeySpace) && u.down) {
		return // panning, not drawing
	}
	inside := m.cur.Contains(x, y)

	switch m.tool {
	case toolBrush, toolErase:
		id := m.brush
		if m.tool == toolErase {
			id = hhm.Empty
		}
		if u.clicked {
			m.snapshot()
		}
		if u.down && inside && m.cur.TileAt(x, y) != id {
			m.cur.SetTile(x, y, id)
			m.touch()
		}
	case toolRect:
		if u.clicked && inside {
			m.rectA, m.rectOn = image.Pt(x, y), true
		}
		if m.rectOn && u.released {
			m.snapshot()
			r := normalize(m.rectA, image.Pt(x, y))
			outline := shiftHeld()
			for j := r.Min.Y; j < r.Max.Y; j++ {
				for i := r.Min.X; i < r.Max.X; i++ {
					if outline && i != r.Min.X && i != r.Max.X-1 && j != r.Min.Y && j != r.Max.Y-1 {
						continue
					}
					m.cur.SetTile(i, j, m.brush)
				}
			}
			m.rectOn = false
			m.touch()
			a.say("filled %dx%d", r.Dx(), r.Dy())
		}
	case toolFill:
		if u.clicked && inside {
			m.snapshot()
			m.floodFill(x, y, m.brush)
			m.touch()
		}
	case toolPick:
		if u.down && inside {
			if id := m.cur.TileAt(x, y); id != hhm.Empty {
				m.brush = id
			}
		}
	case toolEntity:
		if u.clicked && inside {
			m.snapshot()
			e := hhm.NewEntity(hhm.EntityKinds[m.kind].Name, x, y)
			m.cur.Entities = append(m.cur.Entities, e)
			m.sel = e
			m.touch()
		}
		if u.rightClk && inside {
			if es := m.cur.EntitiesAt(x, y); len(es) > 0 {
				m.snapshot()
				m.cur.Remove(es[len(es)-1])
				m.sel = nil
				m.touch()
			}
		}
	case toolSelect:
		if u.clicked {
			if es := m.cur.EntitiesAt(x, y); len(es) > 0 {
				m.sel = es[len(es)-1]
				m.drag = m.sel
				m.snapshot()
				for i, k := range hhm.EntityKinds { // the palette follows the selection
					if k.Name == m.sel.Kind {
						m.kind = i
					}
				}
			} else {
				m.sel = nil
			}
		}
		if m.drag != nil && u.down && inside && (m.drag.X != x || m.drag.Y != y) {
			m.drag.X, m.drag.Y = x, y
			m.touch()
		}
		if !u.down {
			m.drag = nil
		}
		if u.rightClk {
			if es := m.cur.EntitiesAt(x, y); len(es) > 0 {
				m.snapshot()
				m.cur.Remove(es[len(es)-1])
				m.sel = nil
				m.touch()
			}
		}
	}
}

// floodFill paints the connected run of like tiles starting at the cell.
func (m *mapEditor) floodFill(x, y, to int) {
	from := m.cur.TileAt(x, y)
	if from == to {
		return
	}
	queue := []image.Point{{X: x, Y: y}}
	for len(queue) > 0 {
		p := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if !m.cur.Contains(p.X, p.Y) || m.cur.TileAt(p.X, p.Y) != from {
			continue
		}
		m.cur.SetTile(p.X, p.Y, to)
		queue = append(queue, image.Pt(p.X+1, p.Y), image.Pt(p.X-1, p.Y), image.Pt(p.X, p.Y+1), image.Pt(p.X, p.Y-1))
	}
}
