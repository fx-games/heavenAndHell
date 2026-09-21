package dungen

import (
	"io/fs"
	"math/rand"
	"os"
	"testing"

	"rpg/dungeon"
	"rpg/files"
	"rpg/geometry"
	"rpg/recfile"
)

// hnhLevels is the whole chain a mod's level declaration goes through: the overlay finds the mod's levels.rec,
// the engine's registry resolves each generator name, and each algorithm parses its own knobs. Content is read
// here, inside the test, never at package init.
func hnhLevels(t *testing.T) ([]dungeon.Level, fs.FS) {
	t.Helper()
	files.SetFileSystem(os.DirFS("../files").(files.FileSystem))
	files.SelectMod("heavenandhell")
	fsys := files.GetFileSystem()
	f, err := fsys.Open("assets/data/levels.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	levels, err := dungeon.LoadLevels(recfile.ReadMulti(f)["default"], 13)
	if err != nil {
		t.Fatal(err)
	}
	return levels, fsys
}

// TestHeavenAndHellLevelsAreWalkable is the check the whole refactor exists for: every generator name Heaven &
// Hell declares resolves in the engine's registry, builds, and comes out as one connected walkable region. A
// level that splits in two is one the player can fall into half of and never leave.
func TestHeavenAndHellLevelsAreWalkable(t *testing.T) {
	levels, fsys := hnhLevels(t)
	for dlvl := 1; dlvl < len(levels); dlvl++ {
		l := levels[dlvl]
		if l.Generator == "" {
			t.Fatalf("level %d declares no generator", dlvl)
		}
		m, err := Build(l.Config, rand.New(rand.NewSource(int64(dlvl))), fsys)
		if err != nil {
			t.Fatalf("level %d (%s): %v", dlvl, l.Generator, err)
		}
		assertConnected(t, dlvl, l.Generator, m)
	}
}

// TestMegaDungeonIsWalkable covers the other registered algorithm, which no level declares yet: its defaults
// still have to produce a level someone could walk through.
func TestMegaDungeonIsWalkable(t *testing.T) {
	cfg, err := megadungeon{}.Parse(recfile.Record{{Name: "generator", Value: "megadungeon"}})
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= 3; seed++ {
		m, err := Build(cfg, rand.New(rand.NewSource(seed)), nil)
		if err != nil {
			t.Fatal(err)
		}
		assertConnected(t, int(seed), "megadungeon", m)
	}
}

// TestSameSeedSameLevel pins the determinism the playthrough depends on: these generators gather candidates out
// of maps keyed by pointers, so without a sort the same seed used to build two different levels.
func TestSameSeedSameLevel(t *testing.T) {
	cfg, err := megadungeon{}.Parse(recfile.Record{{Name: "generator", Value: "megadungeon"}})
	if err != nil {
		t.Fatal(err)
	}
	build := func() string {
		m, err := Build(cfg, rand.New(rand.NewSource(7)), nil)
		if err != nil {
			t.Fatal(err)
		}
		w, h := m.GetSize()
		out := make([]byte, 0, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out = append(out, byte('0'+m.GetTile(x, y)))
			}
		}
		return string(out)
	}
	if a, b := build(), build(); a != b {
		t.Fatal("the same seed built two different levels")
	}
}

// TestUnknownGeneratorIsRejected: a typo in a mod's levels.rec fails when the mod loads, not on the stairs down.
func TestUnknownGeneratorIsRejected(t *testing.T) {
	_, err := dungeon.LoadLevels([]recfile.Record{{{Name: "dlvl", Value: "1"}, {Name: "generator", Value: "acretion"}}}, 2)
	if err == nil {
		t.Fatal("an unknown generator name loaded")
	}
}

// assertConnected floods the walkable cells from the first one and insists it reaches all of them.
func assertConnected(t *testing.T, dlvl int, name string, m *DungeonMap) {
	t.Helper()
	w, h := m.GetSize()
	var start geometry.Point
	total, found := 0, false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if m.IsWalkable(geometry.Point{X: x, Y: y}) {
				total++
				if !found {
					start, found = geometry.Point{X: x, Y: y}, true
				}
			}
		}
	}
	if total < 50 {
		t.Errorf("level %d (%s): only %d walkable cells", dlvl, name, total)
		return
	}
	seen := map[geometry.Point]bool{start: true}
	queue := []geometry.Point{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, n := range m.GetCardinalNeighbours(p) {
			if !seen[n] && m.IsWalkable(n) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	if len(seen) != total {
		t.Errorf("level %d (%s): %d of %d walkable cells are cut off from the rest", dlvl, name, total-len(seen), total)
	}
}
