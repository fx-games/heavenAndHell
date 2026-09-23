package dungen

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"rpg/src/dungeon"
	"rpg/src/recfile"
	"rpg/src/rules"
)

// D1's four level builders, each an algorithm plus the tile set it draws with. package common registers each as a
// LevelStrategy under its name (and D1Set as d1_set); the knobs a mod may turn are LevelParams.
type strategy struct {
	ltype int                        // the tile set and theme rules the algorithm draws with
	fill  int                        // lPass3's background megatile
	build func(g *gen, entry Entry)  // the algorithm
	deco  func(g *gen, d Decoration) // how this algorithm scatters single-tile decorations
	name  string
}

// D1 holds the four strategies by name; D1Set is the quest set levels'.
var D1 = map[string]dungeon.Parser{}

var D1Set dungeon.Parser = setStrategy{}

func init() {
	for name, st := range map[string]strategy{
		"d1_cathedral": {rules.DTypeCathedral, t1Dirt - 1, (*gen).l1GenerateLevel, (*gen).l2PlaceDecoration, ""},
		"d1_catacombs": {rules.DTypeCatacombs, 12 - 1, (*gen).l2GenerateLevel, (*gen).l2PlaceDecoration, ""},
		"d1_caves":     {rules.DTypeCaves, 8 - 1, (*gen).l3GenerateLevel, (*gen).l3PlaceDecoration, ""},
		"d1_hell":      {rules.DTypeHell, 30 - 1, (*gen).l4GenerateLevel, (*gen).l2PlaceDecoration, ""},
	} {
		st.name = name
		D1[name] = st
	}
}

// SetLevelParams is a d1_set level: the quest whose set piece it loads (LoadSetMap). The set level number and
// its leveltype come from 1.09's own quest table, which is this family's knowledge, not the mod's.
type SetLevelParams struct{ Quest string }

type setStrategy struct{}

func (setStrategy) Parse(r recfile.Record) (dungeon.Config, error) {
	q, ok := dungeon.Field(r, "quest")
	if !ok || q == "" {
		return nil, fmt.Errorf("levels.rec: d1_set needs a quest")
	}
	return SetLevelParams{Quest: q}, nil
}

// strategyNamed is D1's strategy called name, or the zero strategy.
func strategyNamed(name string) strategy {
	st, _ := D1[name].(strategy)
	return st
}

// Decoration is one single-tile substitution a strategy scatters over a finished level: tile Search becomes tile
// Replace on Freq percent of the cells that carry it. Name is for the mod author's benefit only.
type Decoration struct {
	Name            string
	Search, Replace byte
	Freq            int
}

// LevelParams are the knobs a mod may turn on a D1 strategy. Every field defaults to what Diablo does, so a
// levels.rec record that names only a generator reproduces the original level exactly.
type LevelParams struct {
	Decorations []Decoration // which tiles decorate the level, and how often (knobs 1 and 2)
	ThemeFreq   int          // DRLG_PlaceThemeRooms' frequency (knob 2)
	SetPieces   bool         // whether the strategy lays out its set piece at all (knob 3)
}

// defaultParams is what each strategy does in Diablo. Level size is deliberately NOT a parameter: the generators'
// 112x112 and 40x40 arrays are array types, not slices.
var defaultParams = map[string]LevelParams{
	"d1_cathedral": {SetPieces: true},
	"d1_catacombs": {SetPieces: true, Decorations: []Decoration{
		{"cracked floor", 1, 80, 10}, {"pitted floor", 1, 81, 10}, {"stained floor", 1, 82, 10},
		{"cracked wall", 2, 84, 10}, {"pitted wall", 2, 85, 10}, {"stained wall", 2, 86, 10},
		{"rubble", 8, 87, 50},
	}},
	"d1_caves": {SetPieces: true, Decorations: []Decoration{
		{"pebbles", 7, 106, 25}, {"gravel", 7, 107, 25}, {"cracks", 7, 108, 25},
		{"lava crust", 9, 109, 25}, {"lava bubble", 10, 110, 25},
	}},
	"d1_hell": {SetPieces: true, ThemeFreq: 8},
}

// Parse implements dungeon.Parser: it turns one levels.rec record into this strategy's LevelParams. Fields the
// record leaves out keep the strategy's Diablo defaults.
func (st strategy) Parse(r recfile.Record) (dungeon.Config, error) {
	p := defaultParams[st.name]
	var err error
	if p.ThemeFreq, err = dungeon.Int(r, "theme_freq", p.ThemeFreq); err != nil {
		return nil, err
	}
	if v, ok := dungeon.Field(r, "set_pieces"); ok {
		p.SetPieces = v == "true"
	}
	// Any decoration field at all replaces the strategy's default list, so a mod can switch decorations off with
	// a single "decoration: none".
	if decs := dungeon.FieldsNamed(r, "decoration"); len(decs) > 0 {
		p.Decorations = nil
		for _, d := range decs {
			if d == "none" {
				continue
			}
			dec, err := parseDecoration(d)
			if err != nil {
				return nil, err
			}
			p.Decorations = append(p.Decorations, dec)
		}
	}
	return p, nil
}

// levelConfig is one dungeon level's entry in levels.rec.
type levelConfig struct {
	strategy string
	params   LevelParams
}

// levelConfigs is a mod's whole level list, indexed by dungeon level.
type levelConfigs [rules.NumLevels]levelConfig

// loadLevelConfigs reads the D1 levels of levels.rec. Called from Load, never at package init:
// content only exists after a mod is selected.
func loadLevelConfigs(fsys fs.FS) (levelConfigs, error) {
	var cfgs levelConfigs
	recs, err := rules.ReadRec(fsys, "levels.rec")
	if err != nil {
		return cfgs, err
	}
	// D1's generator reads its knobs by depth: the one D1 dungeon level each depth has. The whole file is checked
	// by common's loader (dungeon.LoadLevels); other generators' levels, d1_set's included (they build with their
	// parent depth's), are not its business here.
	for _, r := range recs {
		gen, _ := dungeon.Field(r, "generator")
		st, ok := D1[gen]
		if !ok {
			continue
		}
		cfg, err := st.Parse(r)
		if err != nil {
			return cfgs, err
		}
		depth, err := dungeon.Int(r, "depth", 0)
		if err != nil {
			return cfgs, err
		}
		if p, ok := cfg.(LevelParams); ok && depth > 0 && depth < rules.NumLevels {
			cfgs[depth] = levelConfig{strategy: gen, params: p}
		}
	}
	return cfgs, nil
}

// parseDecoration reads "name, search tile, replace tile, percent".
func parseDecoration(s string) (Decoration, error) {
	p := strings.Split(s, ",")
	if len(p) != 4 {
		return Decoration{}, fmt.Errorf("levels.rec: decoration %q: want name, search, replace, percent", s)
	}
	var n [3]int
	for i := range n {
		v, err := strconv.Atoi(strings.TrimSpace(p[i+1]))
		if err != nil {
			return Decoration{}, fmt.Errorf("levels.rec: decoration %q: %w", s, err)
		}
		n[i] = v
	}
	return Decoration{strings.TrimSpace(p[0]), byte(n[0]), byte(n[1]), n[2]}, nil
}
