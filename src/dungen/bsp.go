package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"

	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/recfile"
)

func init() { Layouts["bsp"] = bsp{} }

// BSPConfig: binary space partitioning. The grid is split in two along its longer side until the parts are too
// small to split, each leaf gets one room, and each split is bridged by a corridor between its halves' rooms.
type BSPConfig struct {
	Width, Height int
	MinLeaf       int // a part is split only while both halves stay at least this big
	MinRoom       int // smallest room floor side
}

type bsp struct{}

func (bsp) Parse(r recfile.Record) (dungeon.Config, error) {
	c := BSPConfig{Width: 80, Height: 50, MinLeaf: 10, MinRoom: 4}
	if err := parseKnobs(r, []intKnob{{"width", &c.Width}, {"height", &c.Height}, {"min_leaf", &c.MinLeaf},
		{"min_room", &c.MinRoom}}, nil); err != nil {
		return nil, err
	}
	if c.MinRoom < 2 || c.MinLeaf < c.MinRoom+2 {
		return nil, fmt.Errorf("levels.rec: bsp needs min_room >= 2 and min_leaf >= min_room+2")
	}
	return c, checkSize("bsp", c.Width, c.Height, c.MinLeaf)
}

func (c BSPConfig) Build(rnd *rand.Rand, _ fs.FS) (*DungeonMap, error) {
	m := NewDungeonMap(c.Width, c.Height)
	var split func(leaf geometry.Rect) []*DungeonRoom
	split = func(leaf geometry.Rect) []*DungeonRoom {
		w, h := leaf.Size().X, leaf.Size().Y
		horizontal := h > w || h == w && rnd.Intn(2) == 0
		side := w
		if horizontal {
			side = h
		}
		if side < 2*c.MinLeaf {
			// A leaf includes the walls around its room: one cell each side.
			rw, rh := c.MinRoom+rnd.Intn(w-c.MinRoom-1), c.MinRoom+rnd.Intn(h-c.MinRoom-1)
			x, y := leaf.Min.X+1+rnd.Intn(w-rw-1), leaf.Min.Y+1+rnd.Intn(h-rh-1)
			return []*DungeonRoom{addRectRoom(m, rnd, geometry.NewRect(x, y, x+rw, y+rh))}
		}
		cut := c.MinLeaf + rnd.Intn(side-2*c.MinLeaf+1)
		a, b := leaf, leaf
		if horizontal {
			a.Max.Y, b.Min.Y = leaf.Min.Y+cut, leaf.Min.Y+cut
		} else {
			a.Max.X, b.Min.X = leaf.Min.X+cut, leaf.Min.X+cut
		}
		ra, rb := split(a), split(b)
		carveL(m, rnd, center(ra[rnd.Intn(len(ra))]), center(rb[rnd.Intn(len(rb))]))
		return append(ra, rb...)
	}
	split(geometry.NewRect(0, 0, c.Width, c.Height))
	return m, nil
}
