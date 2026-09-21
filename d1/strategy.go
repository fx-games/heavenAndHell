package d1

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"rpg/recfile"
)

// Dungeon generation is one subsystem with several named strategies. A strategy is an algorithm plus the tile set
// it draws with; a mod says which strategy builds which dungeon level, and with which parameters, in levels.rec.
// The registry is a plain map: a name in, a way to build a level out. Nothing registers itself at init time and
// nothing here knows which mod is loaded.
type strategy struct {
	ltype int                        // the tile set and theme rules the algorithm draws with
	fill  int                        // lPass3's background megatile
	build func(g *gen, entry Entry)  // the algorithm
	deco  func(g *gen, d Decoration) // how this algorithm scatters single-tile decorations
}

var strategies = map[string]strategy{
	"cathedral": {DTypeCathedral, t1Dirt - 1, (*gen).l1GenerateLevel, (*gen).l2PlaceDecoration},
	"catacombs": {DTypeCatacombs, 12 - 1, (*gen).l2GenerateLevel, (*gen).l2PlaceDecoration},
	"caves":     {DTypeCaves, 8 - 1, (*gen).l3GenerateLevel, (*gen).l3PlaceDecoration},
	"hell":      {DTypeHell, 30 - 1, (*gen).l4GenerateLevel, (*gen).l2PlaceDecoration},
}

// Decoration is one single-tile substitution a strategy scatters over a finished level: tile Search becomes tile
// Replace on Freq percent of the cells that carry it. Name is for the mod author's benefit only.
type Decoration struct {
	Name            string
	Search, Replace byte
	Freq            int
}

// LevelParams are the knobs a mod may turn on a strategy. Every field defaults to what Diablo does, so a levels.rec
// record that names only a generator reproduces the original level exactly.
type LevelParams struct {
	Decorations []Decoration // which tiles decorate the level, and how often (knobs 1 and 2)
	ThemeFreq   int          // DRLG_PlaceThemeRooms' frequency (knob 2)
	SetPieces   bool         // whether the strategy lays out its set piece at all (knob 3)
}

// defaultParams is what each strategy does in Diablo. Level size is deliberately NOT a parameter: the generators'
// 112x112 and 40x40 arrays are array types, not slices.
var defaultParams = map[string]LevelParams{
	"cathedral": {SetPieces: true},
	"catacombs": {SetPieces: true, Decorations: []Decoration{
		{"cracked floor", 1, 80, 10}, {"pitted floor", 1, 81, 10}, {"stained floor", 1, 82, 10},
		{"cracked wall", 2, 84, 10}, {"pitted wall", 2, 85, 10}, {"stained wall", 2, 86, 10},
		{"rubble", 8, 87, 50},
	}},
	"caves": {SetPieces: true, Decorations: []Decoration{
		{"pebbles", 7, 106, 25}, {"gravel", 7, 107, 25}, {"cracks", 7, 108, 25},
		{"lava crust", 9, 109, 25}, {"lava bubble", 10, 110, 25},
	}},
	"hell": {SetPieces: true, ThemeFreq: 8},
}

// levelConfig is one dungeon level's entry in levels.rec.
type levelConfig struct {
	strategy string
	params   LevelParams
}

// levelConfigs is a mod's whole level list, indexed by dungeon level.
type levelConfigs [NumLevels]levelConfig

// loadLevelConfigs reads levels.rec. Called from Load, never at package init: content only exists after a mod is
// selected.
func loadLevelConfigs(fsys fs.FS) (levelConfigs, error) {
	var cfgs levelConfigs
	recs, err := readRec(fsys, "levels.rec")
	if err != nil {
		return cfgs, err
	}
	for _, r := range recs {
		cfg, dlvl, err := parseLevelConfig(r)
		if err != nil {
			return cfgs, err
		}
		if dlvl < 1 || dlvl >= NumLevels {
			return cfgs, fmt.Errorf("levels.rec: dlvl %d out of range", dlvl)
		}
		cfgs[dlvl] = cfg
	}
	for dlvl := 1; dlvl < NumLevels; dlvl++ {
		if cfgs[dlvl].strategy == "" {
			return cfgs, fmt.Errorf("levels.rec: no generator for dungeon level %d", dlvl)
		}
	}
	return cfgs, nil
}

// parseLevelConfig turns one record into a level's strategy and parameters. Fields the record leaves out keep the
// strategy's Diablo defaults.
func parseLevelConfig(r recfile.Record) (levelConfig, int, error) {
	m := r.ToMap("")
	name := m.GetStringOrDefault("generator", "")
	if _, ok := strategies[name]; !ok {
		return levelConfig{}, 0, fmt.Errorf("levels.rec: unknown generator %q", name)
	}
	cfg := levelConfig{strategy: name, params: defaultParams[name]}
	if v, ok := field(r, "theme_freq"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, 0, fmt.Errorf("levels.rec: theme_freq %q: %w", v, err)
		}
		cfg.params.ThemeFreq = n
	}
	if v, ok := field(r, "set_pieces"); ok {
		cfg.params.SetPieces = v == "true"
	}
	// Any decoration field at all replaces the strategy's default list, so a mod can switch decorations off with
	// a single "decoration: none".
	if decs := fieldsNamed(r, "decoration"); len(decs) > 0 {
		cfg.params.Decorations = nil
		for _, d := range decs {
			if d == "none" {
				continue
			}
			dec, err := parseDecoration(d)
			if err != nil {
				return cfg, 0, err
			}
			cfg.params.Decorations = append(cfg.params.Decorations, dec)
		}
	}
	dlvl, err := strconv.Atoi(m.GetStringOrDefault("dlvl", ""))
	if err != nil {
		return cfg, 0, fmt.Errorf("levels.rec: dlvl: %w", err)
	}
	return cfg, dlvl, nil
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

// field is the first value a record carries under key.
func field(r recfile.Record, key string) (string, bool) {
	for _, f := range r {
		if f.Name == key {
			return strings.TrimSpace(f.Value), true
		}
	}
	return "", false
}
