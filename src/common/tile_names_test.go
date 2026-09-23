package common

import (
	"testing"

	"rpg/files"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/hhm"
)

// TestD1TilesCarryTheirNames: a D1 level's cells are its megatile map parts' tiles, and each carries its tiles.rec
// record's name, so the hover text says Arch or Grate, not just wall or floor.
func TestD1TilesCarryTheirNames(t *testing.T) {
	withMod(t, "d1")
	levels := loadModLevels(t)
	d1, err := dungen.LoadLevels(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	maps, err := hhm.Load(files.GetFileSystem(), hhm.Dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for seed := int64(1); seed <= 3; seed++ {
		g := modGenerator(t, levels, seed)
		g.factory.Levels, g.factory.State.drawnMaps = d1, maps
		for _, id := range []string{"cathedral_1", "cathedral_2", "cathedral_3", "cathedral_4"} {
			m := g.GenerateMap(id)
			w, h := m.MapSize().X, m.MapSize().Y
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					seen[m.GetCell(geometry.Point{X: x, Y: y}).TileType.DefinedDescription] = true
				}
			}
		}
	}
	for _, want := range []string{"Cathedral Wall", "Cathedral Trap Wall", "Cathedral Floor", "Arch", "Grate"} {
		if !seen[want] {
			t.Errorf("no cathedral cell is called %q (have %v)", want, seen)
		}
	}
}
