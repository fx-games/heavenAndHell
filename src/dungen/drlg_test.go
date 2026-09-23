package dungen

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"rpg/src/rules"
	"strings"
	"testing"
)

type questOverride struct {
	quest int
	field string
	value int
}

type fixtureEntry struct {
	entry Entry
	x, y  int
}

type drlgFixture struct {
	file      string
	dlvl      int
	seed      int32
	overrides []questOverride
	entries   []fixtureEntry
}

const fixtureDir = "../../reference/devilutionX/test/fixtures/"

// TestDrlgFixtures checks the generators against levels D1 builds from fixed seeds (DevilutionX's fixtures:
// megatiles after CreateDungeon and room regions).
func TestDrlgFixtures(t *testing.T) {
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Skip("DevilutionX fixtures not in reference/")
	}
	d, err := rules.Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	// DevilutionX's tests read set pieces from its fixtures, which differ from ours for some Hell pieces.
	readDunHook = func(name string) []byte {
		dir, file, _ := strings.Cut(name, "/") // "l1/sklkng1" -> levels/l1data/sklkng1.dun
		b, err := os.ReadFile(filepath.Join(fixtureDir, "levels", dir+"data", file+".dun"))
		if err != nil {
			return nil
		}
		return b
	}
	defer func() { readDunHook = nil }()
	for _, f := range drlgFixtures {
		if f.file == "diablo/7-1607627156.dun" {
			continue // DevilutionX opens the Halls of the Blind from Blind1.DUN; D1 1.09 starts from Blind2.DUN
		}
		b, err := os.ReadFile(fixtureDir + f.file)
		if err != nil {
			t.Fatal(err)
		}
		want := func(layer, i int) int { return int(binary.LittleEndian.Uint16(b[4+2*(layer+i):])) }
		q := d.InitQuests([rules.NumLevels]int32{})
		for _, o := range f.overrides {
			if o.field == "level" {
				q[o.quest].Level = o.value
			} else {
				q[o.quest].Active = o.value
			}
		}
		for _, e := range f.entries {
			cfg := testLevels(t).cfg[f.dlvl]
			g := &gen{quests: &q, currlevel: f.dlvl, ltype: testLevels(t).LevelType(f.dlvl),
				strat: strategyNamed(cfg.strategy), params: cfg.params}
			g.data, g.levels = d, testLevels(t)
			g.createDungeon(f.seed, e.entry)
			if g.ltype == rules.DTypeHell && len(g.themes) > 0 {
				g.updateL4Trans()
			}
			bad := 0
			for y := 0; y < 40 && bad < 5; y++ {
				for x := 0; x < 40 && bad < 5; x++ {
					if got, w := g.d(x, y), want(0, y*40+x); int(got) != w {
						t.Errorf("%s entry %d: tile %d,%d = %d, want %d", f.file, e.entry, x, y, got, w)
						bad++
					}
				}
			}
			for y := 0; y < 80 && bad < 5; y++ {
				for x := 0; x < 80 && bad < 5; x++ {
					if got, w := uint8(g.trans[16+x][16+y]), want(0, 1600*13+y*80+x); int(got) != w {
						t.Errorf("%s entry %d: trans %d,%d = %d, want %d", f.file, e.entry, 16+x, 16+y, got, w)
						bad++
					}
				}
			}
			if f.seed == 1583642716 && (q[rules.QBetrayer].X != 84 || q[rules.QBetrayer].Y != 64) {
				t.Errorf("%s: Lazarus portal at %d,%d, want 84,64", f.file, q[rules.QBetrayer].X, q[rules.QBetrayer].Y)
			}
			if g.view != (point{e.x, e.y}) {
				t.Errorf("%s entry %d: view %v, want %d,%d", f.file, e.entry, g.view, e.x, e.y)
			}
		}
	}
}
