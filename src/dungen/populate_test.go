package dungen

import (
	"math/rand"
	"strings"
	"testing"

	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/hhm"
	"rpg/src/rules"
)

// standIns are the d1 mod's dungeon levels by depth, as levels.rec declares them (layout, passes, population).
func standIns(t testing.TB) map[int]dungeon.Config {
	t.Helper()
	recs, err := rules.ReadRec(testFS(), "levels.rec")
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]dungeon.Config{}
	for _, r := range recs {
		if _, ok := dungeon.Field(r, "population"); !ok {
			continue
		}
		gen, _ := dungeon.Field(r, "generator")
		cfg, err := WithPasses(Layouts[gen]).Parse(r)
		if err != nil {
			t.Fatal(err)
		}
		depth, _ := dungeon.Int(r, "depth", 0)
		out[depth] = cfg
	}
	return out
}

// TestPopulateStandIns builds every dungeon level of several games the way the game does (quests rolled from the
// seed, their set pieces laid in, D1's population) and checks what the player needs: every monster, object and
// item on ground the stairs reach, unless a set piece walls it in until it opens; every set piece's spots (quest
// entrances, stairs, uniques) reachable, when they are not in what the piece opens later.
func TestPopulateStandIns(t *testing.T) {
	d, err := rules.Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	levels := standIns(t)
	for game := int32(1); game <= 6; game++ {
		seeds := rules.LevelSeeds(game * 7919)
		q := d.InitQuests(seeds)
		for dlvl := 1; dlvl <= 16; dlvl++ {
			cfg, ok := levels[dlvl]
			if !ok {
				t.Fatalf("levels.rec has no populated level %d", dlvl)
			}
			pop, _ := PopulationOf(cfg)
			cfg = WithQuests(cfg, func(key string) bool {
				for i, k := range rules.QuestKeys {
					if k == key {
						return q[i].Level == dlvl && q[i].Active != rules.QuestNotAvail
					}
				}
				return false
			})
			m, err := cfg.(Layout).Build(rand.New(rand.NewSource(int64(game)*100+int64(dlvl))), testFS())
			if err != nil {
				t.Fatalf("game %d level %d: %v", game, dlvl, err)
			}
			var floor []geometry.Point
			for y := 0; y < LevelSize; y++ {
				for x := 0; x < LevelSize; x++ {
					if p := (geometry.Point{X: x, Y: y}); m.IsEmptySpace(p) {
						floor = append(floor, p)
					}
				}
			}
			up, down := floor[0], floor[len(floor)-1]
			trigs := []Trigger{{up.X + CellOrigin, up.Y + CellOrigin, TrigPrevLevel}, {down.X + CellOrigin, down.Y + CellOrigin, TrigNextLevel}}
			lvl, err := testLevels(t).Populate(rules.NewGenerator(d, game), &q, seeds, dlvl, pop, m, trigs)
			if err != nil {
				t.Fatalf("game %d level %d: %v", game, dlvl, err)
			}
			reach := map[geometry.Point]bool{up: true}
			for queue := []geometry.Point{up}; len(queue) > 0; queue = queue[1:] {
				for _, dir := range geometry.Dirs {
					if n := queue[0].Add(dir); m.IsWalkable(n) && !reach[n] {
						reach[n] = true
						queue = append(queue, n)
					}
				}
			}
			if !reach[down] {
				t.Errorf("game %d level %d: the stairs do not reach each other", game, dlvl)
			}
			inPiece := func(p geometry.Point) bool {
				for _, s := range m.Pieces() {
					if p.X >= s.X && p.Y >= s.Y && p.X < s.X+s.Map.Width && p.Y < s.Y+s.Map.Height {
						return true
					}
				}
				return false
			}
			check := func(x, y int, what string) {
				if x == 0 && y == 0 {
					return // Theme_WeaponRack's second tfitObj3 can fail and leave the rack at 0,0, off the map, as in D1
				}
				if p := (geometry.Point{X: x - CellOrigin, Y: y - CellOrigin}); !reach[p] && !inPiece(p) {
					t.Errorf("game %d level %d: %s at %d,%d is not on floor the stairs reach", game, dlvl, what, p.X, p.Y)
				}
			}
			n := 0
			for _, mon := range lvl.Monsters {
				if mon != nil && !mon.Reserved && !mon.Hidden {
					check(mon.X, mon.Y, "monster "+d.Monsters[mon.Type].Name)
					n++
				}
			}
			if n < 20 {
				t.Errorf("game %d level %d: only %d monsters", game, dlvl, n)
			}
			for _, o := range lvl.Objects {
				if name := d.Objects[o.Kind].Name; o.Kind != rules.ObjTRAPL && o.Kind != rules.ObjTRAPR && !strings.Contains(name, "TORCH") && !o.DoorFlag {
					check(o.X, o.Y, "object "+name)
				}
			}
			for _, it := range lvl.Items {
				check(it.X, it.Y, "item")
			}
			// every spot a piece names is reached, once the piece has opened (its quest walls it in until then)
			open := reachable(t, m, up, true)
			for _, s := range m.Pieces() {
				for _, e := range s.Map.Entities {
					if p := (geometry.Point{X: s.X + e.X, Y: s.Y + e.Y}); e.Kind == "Location" && !open[p] {
						t.Errorf("game %d level %d: %s's %s at %d,%d is out of reach", game, dlvl, s.Map.Name, e.Str("Name"), p.X, p.Y)
					}
				}
			}
		}
	}
}

// reachable is the cells walkable from start, with every set piece opened when opened is set.
func reachable(t testing.TB, m *DungeonMap, start geometry.Point, opened bool) map[geometry.Point]bool {
	proj, err := loadTemplates(testFS(), hhm.Dir)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := hhm.ReadTileRecords(testFS(), "assets/data")
	if err != nil {
		t.Fatal(err)
	}
	walk := func(p geometry.Point) bool {
		for _, s := range m.Pieces() {
			if x, y := p.X-s.X, p.Y-s.Y; opened && s.Open != nil && s.Open.TileAt(x, y) != hhm.Empty {
				door := false
				for _, e := range s.Open.EntitiesAt(x, y) {
					door = door || e.Kind == "Door"
				}
				k := cellKind(hhm.DrawnTile(proj.Tileset, recs, s.Open.TileAt(x, y)).Flags, door)
				return k == Room || k == Door
			}
		}
		return m.IsWalkable(p)
	}
	reach := map[geometry.Point]bool{start: true}
	for queue := []geometry.Point{start}; len(queue) > 0; queue = queue[1:] {
		for _, dir := range geometry.Dirs {
			if n := queue[0].Add(dir); m.Contains(n) && walk(n) && !reach[n] {
				reach[n] = true
				queue = append(queue, n)
			}
		}
	}
	return reach
}
