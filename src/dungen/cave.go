package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"

	"rpg/src/dungeon"
	"rpg/src/recfile"
)

func init() { Layouts["cave"] = cave{} }

// CaveConfig: cellular-automata caves (Brogue's createBlobOnGrid, the 4-5 rule): fill% of the grid starts as
// floor, five smoothing rounds, and the largest connected cave is the level.
type CaveConfig struct {
	Width, Height int
	Fill          int     // percent seeded as floor; 50 gives narrow twisting caves (below that the 4-5 rule starves), 60 open caverns
	MinFloor      float64 // regrow until the cave covers this share of the grid
}

type cave struct{}

func (cave) Parse(r recfile.Record) (dungeon.Config, error) {
	c := CaveConfig{Width: 60, Height: 40, Fill: 55, MinFloor: 0.3}
	if err := parseKnobs(r, []intKnob{{"width", &c.Width}, {"height", &c.Height}, {"fill", &c.Fill}},
		[]floatKnob{{"min_floor", &c.MinFloor}}); err != nil {
		return nil, err
	}
	if c.Fill < 1 || c.Fill > 99 {
		return nil, fmt.Errorf("levels.rec: cave fill must be 1..99, not %d", c.Fill)
	}
	if err := checkSize("cave", c.Width, c.Height, 10); err != nil {
		return nil, err
	}
	return c, checkFraction("cave", "min_floor", c.MinFloor)
}

func (c CaveConfig) Build(rnd *rand.Rand, _ fs.FS) (*DungeonMap, error) {
	for try := 0; try < organicTries; try++ {
		blob := caBlob(rnd, c.Width-2, c.Height-2, c.Fill) // inside a one-cell wall border
		if float64(len(blob)) < c.MinFloor*float64(c.Width*c.Height) {
			continue
		}
		m := NewDungeonMap(c.Width, c.Height)
		for _, p := range blob {
			m.SetRoom(p.X+1, p.Y+1)
		}
		return m, nil
	}
	return nil, fmt.Errorf("dungen: cave never reached min_floor %.2f in %d tries", c.MinFloor, organicTries)
}
