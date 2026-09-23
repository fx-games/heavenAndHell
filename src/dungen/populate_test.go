package dungen

import (
	"math/rand"
	"strings"
	"testing"

	"rpg/src/geometry"
	"rpg/src/recfile"
	"rpg/src/rules"
)

// standIn is levels.rec's record for D1 level type ltype, as the d1 mod declares it.
func standIn(t testing.TB, ltype int) Layout {
	t.Helper()
	recs, err := rules.ReadRec(testFS(), "levels.rec")
	if err != nil {
		t.Fatal(err)
	}
	pop := map[int]string{1: "d1_cathedral", 2: "d1_catacombs", 3: "d1_caves", 4: "d1_hell"}[ltype]
	for _, r := range recs {
		if v, _ := fieldOf(r, "population"); v == pop {
			gen, _ := fieldOf(r, "generator")
			cfg, err := WithPasses(Layouts[gen]).Parse(r)
			if err != nil {
				t.Fatal(err)
			}
			return cfg.(Layout)
		}
	}
	t.Skipf("levels.rec has no level populated as %s", pop)
	return nil
}

func fieldOf(r recfile.Record, k string) (string, bool) {
	for _, f := range r {
		if f.Name == k {
			return strings.TrimSpace(f.Value), true
		}
	}
	return "", false
}

// TestPopulateStandIns lays out each level type's stand-in and populates it by D1's rules: every monster,
// object and item stands on open ground the stairs reach (wall torches and traps on the wall they hang in).
func TestPopulateStandIns(t *testing.T) {
	d, err := rules.Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	for ltype := 1; ltype <= 4; ltype++ {
		layout := standIn(t, ltype)
		for game := int32(1); game <= 8; game++ {
			dlvl := 4*(ltype-1) + 1 + int(game)%3 // not a level whose set piece fills it
			seeds := rules.LevelSeeds(game * 7919)
			q := d.InitQuests(seeds)
			m, err := layout.Build(rand.New(rand.NewSource(int64(game))), nil)
			if err != nil {
				t.Fatal(err)
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
			lvl, err := testLevels(t).Populate(rules.NewGenerator(d, game), &q, seeds, dlvl, Population{ltype}, m, trigs)
			if err != nil {
				t.Fatalf("type %d game %d: %v", ltype, game, err)
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
			check := func(x, y int, what string) {
				if !reach[geometry.Point{X: x - CellOrigin, Y: y - CellOrigin}] {
					t.Errorf("type %d game %d level %d: %s at %d,%d is not on floor the stairs reach", ltype, game, dlvl, what, x-CellOrigin, y-CellOrigin)
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
				t.Errorf("type %d game %d level %d: only %d monsters", ltype, game, dlvl, n)
			}
			for _, o := range lvl.Objects {
				if name := d.Objects[o.Kind].Name; o.Kind != rules.ObjTRAPL && o.Kind != rules.ObjTRAPR && !strings.Contains(name, "TORCH") {
					check(o.X, o.Y, "object "+name)
				}
			}
			for _, it := range lvl.Items {
				check(it.X, it.Y, "item")
			}
		}
	}
}
