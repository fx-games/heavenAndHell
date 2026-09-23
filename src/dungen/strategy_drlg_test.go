package dungen

import (
	"strings"
	"testing"

	"rpg/src/dungeon"
	"rpg/src/recfile"
	"rpg/src/rules"
)

// buildWith builds dungeon level dlvl with cfg substituted for the mod's own record, and returns its hash.
func buildWith(t *testing.T, d *rules.Data, dlvl int, cfg levelConfig) string {
	t.Helper()
	saved := testLevels(t).cfg[dlvl]
	testLevels(t).cfg[dlvl] = cfg
	defer func() { testLevels(t).cfg[dlvl] = saved }()
	seeds := rules.LevelSeeds(12345)
	q := d.InitQuests(seeds)
	lvl, err := testLevels(t).CreateLevel(rules.NewGenerator(d, 1), &q, seeds, dlvl, rules.SLNone, EntryMain)
	if err != nil {
		t.Fatal(err)
	}
	return hashLevel(lvl)
}

func parseOne(t *testing.T, rec string) (levelConfig, int) {
	t.Helper()
	recs := recfile.ReadMulti(strings.NewReader("%rec: default\n\n" + rec))["default"]
	if len(recs) != 1 {
		t.Fatalf("want one record, got %d", len(recs))
	}
	levels, err := dungeon.LoadLevels(recs)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range levels {
		return levelConfig{strategy: l.Generator, params: l.Config.(LevelParams)}, l.Depth
	}
	t.Fatal("no level parsed")
	return levelConfig{}, 0
}

// TestModParamsReachTheGenerator is the end-to-end proof that a knob declared in a mod's levels.rec arrives at the
// generator and changes the level it builds. It goes through the real rec parser, not a hand-built struct.
func TestModParamsReachTheGenerator(t *testing.T) {
	d, err := rules.Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	// The mod's own record for level 6 (a catacombs level), as the file declares it.
	base, dlvl := parseOne(t, "map: catacombs_6\ndepth: 6\ntiles: catacombs\nstart: true\ngenerator: d1_catacombs\n")
	if dlvl != 6 || base.strategy != "d1_catacombs" {
		t.Fatalf("parsed %d/%q", dlvl, base.strategy)
	}
	if got := buildWith(t, d, 6, base); got != buildWith(t, d, 6, testLevels(t).cfg[6]) {
		t.Fatal("a bare record does not reproduce the mod's own level 6")
	}
	want := buildWith(t, d, 6, base)

	for _, tc := range []struct{ name, rec string }{
		{"decorations", "map: catacombs_6\ndepth: 6\ntiles: catacombs\nstart: true\ngenerator: d1_catacombs\ndecoration: mod tile, 1, 80, 100\n"},
		{"no decorations", "map: catacombs_6\ndepth: 6\ntiles: catacombs\nstart: true\ngenerator: d1_catacombs\ndecoration: none\n"},
		{"theme frequency", "map: catacombs_6\ndepth: 6\ntiles: catacombs\nstart: true\ngenerator: d1_catacombs\ntheme_freq: 3\n"},
		{"set pieces off", "map: catacombs_6\ndepth: 6\ntiles: catacombs\nstart: true\ngenerator: d1_catacombs\nset_pieces: false\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := parseOne(t, tc.rec)
			if buildWith(t, d, 6, cfg) == want {
				t.Error("the parameter did not change the generated level")
			}
		})
	}
}

// TestDefaultParamsAreDiablo checks the other half of the contract: every strategy has defaults, and the mod's
// file leans on them rather than restating Diablo's numbers.
func TestDefaultParamsAreDiablo(t *testing.T) {
	for dlvl := 1; dlvl < rules.NumLevels; dlvl++ {
		cfg := testLevels(t).cfg[dlvl]
		st := strategyNamed(cfg.strategy)
		if st.name == "" {
			t.Fatalf("level %d names unregistered generator %q", dlvl, cfg.strategy)
		}
		if st.ltype != testLevels(t).LevelType(dlvl) {
			t.Errorf("level %d: strategy %q is type %d, LevelType says %d", dlvl, cfg.strategy, st.ltype, testLevels(t).LevelType(dlvl))
		}
		if def := defaultParams[cfg.strategy]; len(cfg.params.Decorations) != len(def.Decorations) ||
			cfg.params.ThemeFreq != def.ThemeFreq || cfg.params.SetPieces != def.SetPieces {
			t.Errorf("level %d does not run on %q's defaults: %+v", dlvl, cfg.strategy, cfg.params)
		}
	}
}
