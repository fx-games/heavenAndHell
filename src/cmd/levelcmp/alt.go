package main

import (
	"fmt"
	"math/rand"

	"rpg/files"
	"rpg/src/dungen"
	"rpg/src/dungeon"
	"rpg/src/rules"
)

func buildAlt(ltype int, seed int64) (*grid, error) {
	cfg, err := level(d1Dlvl[ltype])
	if err != nil {
		return nil, err
	}
	dm, err := cfg.(dungen.Layout).Build(rand.New(rand.NewSource(seed)), nil)
	if err != nil {
		return nil, err
	}
	w, h := dm.GetSize()
	g := newGrid(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch dm.GetTile(x, y) {
			case dungen.Room, dungen.Corridor, dungen.Shallows:
				g.set(x, y, cFloor)
			case dungen.Door:
				g.set(x, y, cDoor)
			case dungen.Lava, dungen.Lake:
				g.set(x, y, cWater)
			}
		}
	}
	return g, nil
}

// level is the d1 mod's levels.rec record at that depth, parsed like the game parses it.
func level(depth int) (dungeon.Config, error) {
	files.SelectMod("d1")
	recs, err := rules.ReadRec(files.GetFileSystem(), "levels.rec")
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if d, _ := dungeon.Int(r, "depth", 0); d == depth {
			if _, set := dungeon.Field(r, "quest"); !set {
				gen, _ := dungeon.Field(r, "generator")
				return dungen.WithPasses(dungen.Layouts[gen]).Parse(r)
			}
		}
	}
	return nil, fmt.Errorf("levels.rec has no level at depth %d", depth)
}
