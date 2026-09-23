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

func init() { Layouts["noise"] = noise{} }

// NoiseConfig: two octaves of value noise read as a height map: high ground is rock, low ground is water ringed
// by shallows, the rest is open floor, and the largest open area is the level. It gives wide overland-style
// caverns with lakes, unlike the corridors of everything else here.
type NoiseConfig struct {
	Width, Height int
	Scale         int     // noise cell size: larger gives broader features
	Rock, Water   float64 // heights above Rock are wall, below Water lake
	MinFloor      float64
	Rivers        int     // rivers run out of the lakes to the map edge or another lake (addRivers)
	RiverWidth    int     // their width in cells (default 2)
	RiverMeander  float64 // chance each step of a river turns aside instead of heading for its goal
}

type noise struct{}

func (noise) Parse(r recfile.Record) (dungeon.Config, error) {
	c := NoiseConfig{Width: 80, Height: 50, Scale: 10, Rock: 0.6, Water: 0.3, MinFloor: 0.3, RiverWidth: 2, RiverMeander: 0.3}
	if err := parseKnobs(r, []intKnob{{"width", &c.Width}, {"height", &c.Height}, {"scale", &c.Scale},
		{"rivers", &c.Rivers}, {"river_width", &c.RiverWidth}},
		[]floatKnob{{"rock", &c.Rock}, {"water", &c.Water}, {"min_floor", &c.MinFloor}, {"river_meander", &c.RiverMeander}}); err != nil {
		return nil, err
	}
	if c.Scale < 2 || c.Water >= c.Rock {
		return nil, fmt.Errorf("levels.rec: noise needs scale >= 2 and water below rock")
	}
	if c.Rivers < 0 || c.RiverWidth < 1 {
		return nil, fmt.Errorf("levels.rec: noise needs rivers >= 0 and river_width >= 1")
	}
	if err := checkSize("noise", c.Width, c.Height, 10); err != nil {
		return nil, err
	}
	for _, f := range []struct {
		n string
		v float64
	}{{"rock", c.Rock}, {"water", c.Water}, {"min_floor", c.MinFloor}, {"river_meander", c.RiverMeander}} {
		if err := checkFraction("noise", f.n, f.v); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c NoiseConfig) Build(rnd *rand.Rand, _ fs.FS) (*DungeonMap, error) {
	for try := 0; try < organicTries; try++ {
		fine, coarse := valueNoise(rnd, c.Width, c.Height, c.Scale/2), valueNoise(rnd, c.Width, c.Height, c.Scale)
		m := NewDungeonMap(c.Width, c.Height)
		for y := 1; y < c.Height-1; y++ {
			for x := 1; x < c.Width-1; x++ {
				switch v := (2*coarse[x+y*c.Width] + fine[x+y*c.Width]) / 3; {
				case v < c.Water:
					m.tiles[x+y*c.Width] = Lake
				case v <= c.Rock:
					m.SetRoom(x, y)
				}
			}
		}
		if float64(m.keepLargestFloor()) < c.MinFloor*float64(c.Width*c.Height) {
			continue
		}
		m.addRivers(rnd, c.Rivers, c.RiverWidth, c.RiverMeander)
		for i, t := range m.tiles { // shallows around the water
			p := geometry.Point{X: i % c.Width, Y: i / c.Width}
			if t == Room && slices.ContainsFunc(dirs8, func(d geometry.Point) bool {
				n := p.Add(d)
				return m.Contains(n) && m.GetTileAt(n) == Lake
			}) {
				m.tiles[i] = Shallows
			}
		}
		return m, nil
	}
	return nil, fmt.Errorf("dungen: noise never reached min_floor %.2f in %d tries", c.MinFloor, organicTries)
}

// valueNoise is random values on a lattice every cell cells apart, smoothly interpolated, in 0..1.
func valueNoise(rnd *rand.Rand, w, h, cell int) []float64 {
	lw := w/cell + 2
	lattice := make([]float64, lw*(h/cell+2))
	for i := range lattice {
		lattice[i] = rnd.Float64()
	}
	smooth := func(t float64) float64 { return t * t * (3 - 2*t) }
	out := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gx, gy := x/cell, y/cell
			tx, ty := smooth(float64(x%cell)/float64(cell)), smooth(float64(y%cell)/float64(cell))
			at := func(dx, dy int) float64 { return lattice[gx+dx+(gy+dy)*lw] }
			top := at(0, 0) + (at(1, 0)-at(0, 0))*tx
			bottom := at(0, 1) + (at(1, 1)-at(0, 1))*tx
			out[x+y*w] = top + (bottom-top)*ty
		}
	}
	return out
}

// keepLargestFloor walls in every Room cell outside the largest 4-connected stretch of Room and returns its size.
func (m *DungeonMap) keepLargestFloor() int {
	part := make([]int, len(m.tiles)) // 0: not yet flooded
	sizes := []int{0}
	for i, t := range m.tiles {
		if t != Room || part[i] != 0 {
			continue
		}
		id := len(sizes)
		sizes = append(sizes, 0)
		part[i] = id
		for q := []geometry.Point{{X: i % m.width, Y: i / m.width}}; len(q) > 0; q = q[1:] {
			sizes[id]++
			for _, d := range dirs4 {
				if n := q[0].Add(d); m.Contains(n) && m.GetTileAt(n) == Room && part[n.X+n.Y*m.width] == 0 {
					part[n.X+n.Y*m.width] = id
					q = append(q, n)
				}
			}
		}
	}
	best := 0
	for id := range sizes {
		if sizes[id] > sizes[best] {
			best = id
		}
	}
	for i := range m.tiles {
		if m.tiles[i] == Room && part[i] != best {
			m.tiles[i] = Wall
		}
	}
	return sizes[best]
}
