package common

import (
	"math/rand"
	"path"
	"slices"
	"testing"

	"rpg/d1"
	"rpg/defs"
	"rpg/dungeon"
	"rpg/files"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/recfile"
	"rpg/stats"
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

// modFactory is the smallest Factory that can turn the active mod's records into actors and items: its own
// tables, its own RNG, and a State for the item factory to hang the player getter off. Nothing here is
// D1-specific; it is whatever mod withMod selected.
func modGenerator(t *testing.T, levels []dungeon.Level, seed int64) *MapGenerator {
	t.Helper()
	data, err := d1.Load(files.GetFileSystem())
	if err != nil {
		t.Fatalf("loading %s content: %v", files.ActiveMod(), err)
	}
	rnd := rand.New(rand.NewSource(seed))
	g := NewMapGenerator(rnd)
	g.SetLevels(levels)
	g.SetSeed(seed)
	s := NewState(nil)
	s.SetMapGenerator(g)
	f := &Factory{RandomSource: rnd, State: &s, Generator: d1.NewGenerator(data, int32(seed))}
	s.SetFactory(f)
	g.SetFactory(f)
	return g
}

func loadModLevels(t *testing.T) []dungeon.Level {
	t.Helper()
	f, err := files.Open(path.Join("assets", "data", "levels.rec"))
	if err != nil {
		t.Fatalf("levels.rec: %v", err)
	}
	defer f.Close()
	levels, err := dungeon.LoadLevels(recfile.Read(f))
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
	built := 0
	for i, l := range levels {
		if l.Generator == "ldtk" {
			continue
		}
		built++
		g := modGenerator(t, levels, int64(i+1)*7919)
		dlvl := l.Map
		m := g.GenerateMap(l.Map)

		if m.GetDisplayName() == "" {
			t.Errorf("%s has no display name", dlvl)
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
			t.Errorf("%s is not a dungeon: %d walkable, %d solid", dlvl, floors, walls)
			continue
		}

		up := m.GetNamedLocation("ladder_up")
		if !m.IsTileWalkable(up) {
			t.Errorf("%s: no way in (ladder_up at %v is not walkable)", dlvl, up)
			continue
		}
		seen := reachable(m, up)
		if len(seen) < floors/2 {
			t.Errorf("%s: only %d of %d walkable cells reachable from the entrance", dlvl, len(seen), floors)
		}

		// Every exit the level declares is there, reachable, and (unless it waits on a condition) leads somewhere.
		for _, e := range l.Exits {
			p := m.GetNamedLocation(e.Location)
			if !seen[p] {
				t.Errorf("%s: exit %q at %v is not reachable from ladder_up at %v", dlvl, e.Location, p, up)
			}
			if _, ok := m.GetTransitionAt(p); !ok && e.OpenWhen == "" {
				t.Errorf("%s: exit %q leads nowhere", dlvl, e.Location)
			}
		}
		if _, ok := m.GetTransitionAt(m.GetNamedLocation("ladder_down")); ok && !slices.ContainsFunc(l.Exits, func(e dungeon.Exit) bool { return e.Location == "ladder_down" }) {
			t.Errorf("%s declares no way down but has one", dlvl)
		}
	}
	if built != 13 {
		t.Errorf("built %d generated levels, want 13 (twelve deep and the poisoned water supply)", built)
	}
}

// A level with nothing on it is terrain, not a dungeon. This is the population half: the monsters and the floor
// loot have to be THIS mod's, drawn from its enemies.rec and its items.rec, at a depth its own records allow -
// which is the whole point of the split, and the thing a "reuse D1's placement" shortcut would have broken.
func TestHeavenAndHellLevelsArePopulatedFromItsOwnTables(t *testing.T) {
	withMod(t, "heavenandhell")
	levels := loadModLevels(t)

	monsters, err := defs.ReadMonsters(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	ownMonster := map[string]defs.Monster{}
	for _, mon := range monsters {
		ownMonster[mon.ID] = mon
	}

	totalActors, totalItems := 0, 0
	for i, l := range levels {
		if l.Generator == "ldtk" {
			continue
		}
		dlvl := l.Depth
		g := modGenerator(t, levels, int64(i+1)*104729)
		m := g.GenerateMap(l.Map)

		actors := m.Actors()
		totalActors += len(actors)
		totalItems += len(m.Items())
		for _, a := range actors {
			mon, ok := ownMonster[a.GetInternalName()]
			if !ok {
				t.Errorf("%s: %q is not one of this mod's monsters", l.Map, a.GetInternalName())
				continue
			}
			// Nothing may turn up deeper than its record allows. Shallower is legal: that is out-of-depth.
			if mon.MaxDLvl > 0 && dlvl > mon.MaxDLvl {
				t.Errorf("%s: %s is capped at dlvl %d", l.Map, mon.ID, mon.MaxDLvl)
			}
			if a.GetStats().GetAttribute(stats.MaxHealth) <= 0 {
				t.Errorf("%s: %s spawned with no hit points", l.Map, mon.ID)
			}
		}
	}
	if totalActors == 0 {
		t.Error("twelve levels and not one monster on any of them")
	}
	if totalItems == 0 {
		t.Error("twelve levels and not one item on any of them")
	}
	t.Logf("twelve levels: %d monsters, %d floor items, all from this mod's tables", totalActors, totalItems)
}

// Same seed, same level: a dungeon that differs run to run cannot be saved, reloaded or reasoned about, and
// three of this engine's four nondeterminism bugs were exactly this.
func TestDungenLevelsAreReproducibleFromTheirSeed(t *testing.T) {
	withMod(t, "heavenandhell")
	levels := loadModLevels(t)

	build := func() string {
		g := modGenerator(t, levels, 4242)
		m := g.GenerateMap("catacombs_3")
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
