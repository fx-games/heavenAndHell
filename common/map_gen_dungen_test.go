package common

import (
	"math/rand"
	"path"
	"testing"

	"rpg/defs"
	"rpg/dungeon"
	"rpg/files"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/recfile"
)

// withMod points the whole content stack at one mod for the duration of a test and puts it back afterwards,
// because TestMain set this binary up for D1 and every other test in the package still expects that.
func withMod(t *testing.T, name string) {
	t.Helper()
	files.SelectMod(name)
	_ = defs.LoadIcons() // a mod may still owe the engine an icon; what it DID supply is bound either way
	t.Cleanup(func() {
		files.SelectMod(DefaultMod)
		if err := defs.LoadIcons(); err != nil {
			t.Fatalf("restoring %s icons: %v", DefaultMod, err)
		}
	})
}

func loadModLevels(t *testing.T) []dungeon.Level {
	t.Helper()
	f, err := files.Open(path.Join("assets", "data", "levels.rec"))
	if err != nil {
		t.Fatalf("levels.rec: %v", err)
	}
	defer f.Close()
	levels, err := dungeon.LoadLevels(recfile.Read(f), 0)
	if err != nil {
		t.Fatalf("levels.rec: %v", err)
	}
	return levels
}

// reachable flood-fills the walkable cells from a start, through doors: a door cell is floor with a door object
// standing on it, so walkability alone answers "can the hero get there".
func reachable(m *gridmap.GridMap[*Actor, *Item, Object], from geometry.Point) map[geometry.Point]bool {
	seen := map[geometry.Point]bool{from: true}
	queue := []geometry.Point{from}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, n := range m.GetFilteredNeighbors(p, m.Contains) {
			if !seen[n] && m.IsTileWalkable(n) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return seen
}

// The point of the whole dispatch: selecting a mod whose levels name a dungen generator has to give you a
// gridmap you could actually walk around in - floors, walls, a way in, a way on, and the way on reachable from
// the way in. Before this, GenerateMap was hardwired to D1's generators and these levels could not be entered
// at all.
//
// Every level the mod declares is built, because "level 7 hangs" is exactly the kind of thing that only shows
// up on the level nobody generated.
func TestHeavenAndHellLevelsAreWalkable(t *testing.T) {
	withMod(t, "heavenandhell")
	levels := loadModLevels(t)
	if len(levels) != 13 { // dlvl 0 (town) + twelve dungeon levels
		t.Fatalf("mod declares %d levels, want 13 (town + 12)", len(levels))
	}

	for dlvl := 1; dlvl < len(levels); dlvl++ {
		g := NewMapGenerator(rand.New(rand.NewSource(int64(dlvl) * 7919)))
		g.SetLevels(levels)
		name := defs.LevelNameFromDepth(dlvl)
		m := g.GenerateMap(name, dlvl)

		if m.GetDisplayName() == "" {
			t.Errorf("dlvl %d has no display name", dlvl)
		}
		var floors, walls int
		width, height := m.GetWidth(), m.GetHeight()
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if m.IsTileWalkable(geometry.Point{X: x, Y: y}) {
					floors++
				} else {
					walls++
				}
			}
		}
		if floors == 0 || walls == 0 {
			t.Errorf("dlvl %d is not a dungeon: %d walkable, %d solid", dlvl, floors, walls)
			continue
		}

		up := m.GetNamedLocation("ladder_up")
		if !m.IsTileWalkable(up) {
			t.Errorf("dlvl %d: no way in (ladder_up at %v is not walkable)", dlvl, up)
			continue
		}
		seen := reachable(m, up)
		if len(seen) < floors/2 {
			t.Errorf("dlvl %d: only %d of %d walkable cells reachable from the entrance", dlvl, len(seen), floors)
		}

		// Every level but the last has a way down, and it must be one you can walk to.
		if dlvl+1 < len(levels) {
			down := m.GetNamedLocation("ladder_down")
			if !seen[down] {
				t.Errorf("dlvl %d: ladder_down at %v is not reachable from ladder_up at %v", dlvl, down, up)
			}
			if _, ok := m.GetTransitionAt(down); !ok {
				t.Errorf("dlvl %d: ladder_down leads nowhere", dlvl)
			}
		} else if _, ok := m.GetTransitionAt(m.GetNamedLocation("ladder_down")); ok {
			t.Errorf("dlvl %d is the deepest level the mod declares but still has a way down", dlvl)
		}

		// And every extra way off the level the mod declared is there, and reachable.
		for _, tr := range levels[dlvl].Transitions {
			p := m.GetNamedLocation(tr.LocationName)
			if !seen[p] {
				t.Errorf("dlvl %d: declared transition %q at %v is not reachable", dlvl, tr.LocationName, p)
			}
		}
	}
}

// Same seed, same level: a dungeon that differs run to run cannot be saved, reloaded or reasoned about, and
// three of this engine's four nondeterminism bugs were exactly this.
func TestDungenLevelsAreReproducibleFromTheirSeed(t *testing.T) {
	withMod(t, "heavenandhell")
	levels := loadModLevels(t)

	build := func() string {
		g := NewMapGenerator(rand.New(rand.NewSource(4242)))
		g.SetLevels(levels)
		m := g.GenerateMap(defs.LevelNameFromDepth(3), 3)
		out := make([]byte, 0, m.GetWidth()*m.GetHeight())
		for y := 0; y < m.GetHeight(); y++ {
			for x := 0; x < m.GetWidth(); x++ {
				p := geometry.Point{X: x, Y: y}
				c := byte('#')
				if m.IsObjectAt(p) {
					c = '+'
				} else if m.IsTileWalkable(p) {
					c = '.'
				}
				out = append(out, c)
			}
		}
		return string(out)
	}
	if a, b := build(), build(); a != b {
		t.Error("the same seed built two different levels")
	}
}
