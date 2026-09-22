package dungen

import (
	"io/fs"
	"math/rand"
	"os"
	"testing"

	"rpg/files"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/recfile"
)

// hnhLevels is the whole chain a mod's level declaration goes through: the overlay finds the mod's levels.rec,
// the engine's registry resolves each generator name, and each algorithm parses its own knobs. Content is read
// here, inside the test, never at package init.
func hnhLevels(t *testing.T) ([]dungeon.Level, fs.FS) {
	t.Helper()
	files.SetFileSystem(os.DirFS("../../files").(files.FileSystem))
	files.SelectMod("heavenandhell")
	fsys := files.GetFileSystem()
	f, err := fsys.Open("assets/data/levels.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// Only this package's generators are registered here; the whole file is checked where every generator is
	// (common's mod loading test).
	var levels []dungeon.Level
	for _, r := range recfile.ReadMulti(f)["default"] {
		gen, _ := dungeon.Field(r, "generator")
		st, ok := dungeon.Lookup(gen)
		if !ok {
			continue
		}
		cfg, err := st.Parse(r)
		if err != nil {
			t.Fatal(err)
		}
		depth, _ := dungeon.Int(r, "depth", 0)
		id, _ := dungeon.Field(r, "map")
		levels = append(levels, dungeon.Level{Map: id, Depth: depth, Generator: gen, Config: cfg})
	}
	return levels, fsys
}

// TestHeavenAndHellLevelsAreWalkable is the check the whole refactor exists for: every generator name Heaven &
// Hell declares resolves in the engine's registry, builds, and comes out as one connected walkable region. A
// level that splits in two is one the player can fall into half of and never leave.
func TestHeavenAndHellLevelsAreWalkable(t *testing.T) {
	levels, fsys := hnhLevels(t)
	built := 0
	for _, l := range levels {
		switch l.Config.(type) {
		case AccretionConfig, *AccretionConfig, MegaDungeonConfig, *MegaDungeonConfig:
		default:
			continue
		}
		// Every level, several seeds: a split shows up on some seeds only (the cave levels' connectors did).
		for seed := int64(1); seed <= 100; seed++ {
			m, err := Build(l.Config, rand.New(rand.NewSource(seed)), fsys)
			if err != nil {
				t.Fatalf("level %s (%s): %v", l.Map, l.Generator, err)
			}
			assertConnected(t, int(seed), l.Map, m)
		}
		built++
	}
	if built == 0 {
		t.Fatal("no dungen level declared")
	}
}

// TestMegaDungeonIsWalkable covers the other registered algorithm, which no level declares yet: its defaults
// still have to produce a level someone could walk through.
func TestMegaDungeonIsWalkable(t *testing.T) {
	cfg, err := megadungeon{}.Parse(recfile.Record{{Name: "generator", Value: "megadungeon"}})
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= 100; seed++ {
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
	_, err := dungeon.LoadLevels([]recfile.Record{{{Name: "map", Value: "a"}, {Name: "depth", Value: "1"}, {Name: "tiles", Value: "caves"}, {Name: "start", Value: "true"}, {Name: "generator", Value: "acretion"}}})
	if err == nil {
		t.Fatal("an unknown generator name loaded")
	}
}

// assertConnected floods the walkable cells from the first one and insists it reaches all of them.
func assertConnected(t *testing.T, seed int, name string, m *DungeonMap) {
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
		t.Errorf("seed %d (%s): only %d walkable cells", seed, name, total)
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
		t.Errorf("seed %d (%s): %d of %d walkable cells are cut off from the rest", seed, name, total-len(seen), total)
	}
}

// TestMegaDungeonRejectsBadKnobs: a room bigger than the level used to panic rand.Intn mid-generation.
func TestMegaDungeonRejectsBadKnobs(t *testing.T) {
	for _, kv := range [][2]string{{"width", "8"}, {"min_room_size", "2"}, {"room_tries", "0"},
		{"straight_passage_chance", "1.5"}, {"imperfect_connect_chance", "-1"}, {"room_size_spread", "0"}} {
		r := recfile.Record{{Name: "generator", Value: "megadungeon"}, {Name: kv[0], Value: kv[1]}}
		if _, err := (megadungeon{}).Parse(r); err == nil {
			t.Errorf("%s: %s was accepted", kv[0], kv[1])
		}
	}
}

func TestAccretionRejectsBadLoopChance(t *testing.T) {
	r := recfile.Record{{Name: "generator", Value: "accretion"}, {Name: "template_prefix", Value: "Cath"},
		{Name: "loop_chance", Value: "2"}}
	if _, err := (accretion{}).Parse(r); err == nil {
		t.Error("loop_chance 2 was accepted")
	}
}

// TestMegaDungeonWithoutDoors: doors: false opens the connectors instead, and the level stays one piece.
func TestMegaDungeonWithoutDoors(t *testing.T) {
	cfg, err := megadungeon{}.Parse(recfile.Record{{Name: "generator", Value: "megadungeon"}, {Name: "doors", Value: "false"}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := Build(cfg, rand.New(rand.NewSource(1)), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tile := range m.tiles {
		if tile == Door {
			t.Fatal("doors: false still placed a door")
		}
	}
	assertConnected(t, 1, "megadungeon", m)
}
