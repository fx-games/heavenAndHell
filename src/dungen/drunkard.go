package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"
	"slices"

	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/recfile"
)

func init() { Layouts["drunkard"] = drunkard{} }

// DrunkardConfig: a digger stumbles through rock until the floor covers Cover of the grid. With DLA set it is
// diffusion-limited aggregation instead: each digger starts anywhere and wanders until it touches the cave, then
// digs only that cell, which grows branching, coral-like tunnels rather than blobs.
type DrunkardConfig struct {
	Width, Height int
	Cover         float64 // share of the grid dug out
	WalkLength    int     // steps before the digger restarts from a random dug cell (walk only)
	DLA           bool
}

type drunkard struct{}

func (drunkard) Parse(r recfile.Record) (dungeon.Config, error) {
	c := DrunkardConfig{Width: 60, Height: 40, Cover: 0.35, WalkLength: 200}
	if err := parseKnobs(r, []intKnob{{"width", &c.Width}, {"height", &c.Height}, {"walk_length", &c.WalkLength}},
		[]floatKnob{{"cover", &c.Cover}}); err != nil {
		return nil, err
	}
	if v, ok := dungeon.Field(r, "dla"); ok {
		c.DLA = v == "true"
	}
	if c.WalkLength < 1 {
		return nil, fmt.Errorf("levels.rec: drunkard walk_length must be >= 1")
	}
	if err := checkSize("drunkard", c.Width, c.Height, 10); err != nil {
		return nil, err
	}
	if c.Cover <= 0 || c.Cover > 0.8 {
		return nil, fmt.Errorf("levels.rec: drunkard cover must be above 0 and at most 0.8")
	}
	return c, nil
}

func (c DrunkardConfig) Build(rnd *rand.Rand, _ fs.FS) (*DungeonMap, error) {
	m := NewDungeonMap(c.Width, c.Height)
	inside := func(p geometry.Point) bool { return p.X > 0 && p.Y > 0 && p.X < c.Width-1 && p.Y < c.Height-1 }
	step := func(p geometry.Point) geometry.Point {
		if n := p.Add(dirs4[rnd.Intn(4)]); inside(n) {
			return n
		}
		return p
	}
	var dug []geometry.Point
	dig := func(p geometry.Point) {
		if m.GetTileAt(p) == Wall {
			m.SetRoom(p.X, p.Y)
			dug = append(dug, p)
		}
	}
	dig(geometry.Point{X: c.Width / 2, Y: c.Height / 2})
	target := int(c.Cover * float64((c.Width-2)*(c.Height-2)))
	for len(dug) < target {
		if !c.DLA {
			p := dug[rnd.Intn(len(dug))]
			for i := 0; i < c.WalkLength && len(dug) < target; i++ {
				p = step(p)
				dig(p)
			}
			continue
		}
		p := geometry.Point{X: 1 + rnd.Intn(c.Width-2), Y: 1 + rnd.Intn(c.Height-2)}
		// ponytail: particles walk blind, O(area) steps each; aim them at the cave if big maps get slow
		for i := 0; i < c.Width*c.Height && m.GetTileAt(p) == Wall; i++ {
			if slices.ContainsFunc(dirs4, func(d geometry.Point) bool { return m.GetTileAt(p.Add(d)) == Room }) {
				dig(p)
				break
			}
			p = step(p)
		}
	}
	return m, nil
}
