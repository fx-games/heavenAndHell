// Package dungeon is the engine's one dungeon generation registry: a generator name in, an algorithm out.
//
// There is exactly one of these. A mod declares `dlvl:` + `generator:` (+ that generator's own knobs) in
// levels.rec, and the engine resolves the name here. Nothing in this package knows which mod is loaded, and
// nothing here is shaped like any one generator family.
package dungeon

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"rpg/recfile"
)

// Config is one level's parsed generator configuration. The engine stores it and hands it back to the algorithm
// that produced it; only that algorithm looks inside. This is the deliberate seam: D1's generators need D1's tile
// sets, megatile fills and quest state, H&H's need a room budget and a corridor style, and neither family has to
// grow the other's fields to be registered here.
type Config any

// Strategy is one named dungeon generation algorithm.
//
// It is one method wide on purpose. The engine's only generic business with a generator is "does this name
// resolve, and do the knobs the mod declared for it parse" - so that a bad levels.rec fails when the mod loads,
// not on the stairs down. Everything past that - what the algorithm draws with, and what it hands back - belongs
// to its own family behind this interface. Widening this to the union of both families' needs is how one
// abstraction becomes two in a trenchcoat; if a field only one family understands is wanted, it goes in that
// family's Config.
type Strategy interface {
	// Parse turns one level's record from levels.rec into this algorithm's own configuration. Fields the record
	// leaves out take the algorithm's defaults, so a record naming only a generator is valid.
	Parse(recfile.Record) (Config, error)
}

var strategies = map[string]Strategy{}

// Register adds an algorithm under the name mods declare it by. Algorithms are code, not content, so they
// register at init; nothing content-shaped may be read here.
func Register(name string, s Strategy) {
	if _, dup := strategies[name]; dup {
		panic("dungeon: generator " + name + " registered twice")
	}
	strategies[name] = s
}

// Lookup finds the algorithm a mod named.
func Lookup(name string) (Strategy, bool) { s, ok := strategies[name]; return s, ok }

// Names lists every registered generator, sorted. Sorted because unsorted map iteration is how reproducibility
// dies, and because it is what an error message shows the mod author.
func Names() []string {
	out := make([]string, 0, len(strategies))
	for n := range strategies {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Transition is one extra way off a level, beyond the stairs up and down every dungeon level gets: a named spot
// on this level and where stepping on it leads. Declared as "transition: <name>, <target map>, <target
// location>, up|down".
type Transition struct {
	LocationName   string
	TargetMap      string
	TargetLocation string
	Up             bool
}

// Level is one dungeon level as a mod declared it: which generator builds it, that generator's configuration,
// and the handful of knobs that belong to the LEVEL rather than to any one generator - what it is called, what
// it is drawn with, and where else it leads.
//
// These three are deliberately NOT in Config. Config is a generator family's private business and the engine
// never looks inside it; a display name is the engine's own business whichever algorithm carved the walls.
type Level struct {
	Generator   string
	Config      Config
	Name        string // shown on arrival; empty lets the engine name the level
	Tiles       string // a defs tile set name; empty lets the engine pick by depth
	Transitions []Transition
	// UpTarget overrides where this level's OWN ladder up leads. By default it leads to the level above, and
	// the first level's to the town - but only the mod knows what its town calls that doorway, and the engine
	// may not name it. Declared as "up: <target map>, <target location>".
	UpTarget *Transition
	// Map names a side level: one reached by a declared transition rather than by depth, like a quest's cave.
	// It has a way up (UpTarget) and no way down; Depth is the dungeon level whose monsters and loot it draws on.
	Map   string
	Depth int
}

// LoadLevels reads a mod's whole level list. Levels are indexed by dungeon level, so index 0 (town) stays empty
// and every level from 1 to numLevels-1 must be declared. Called after mod selection, never at package init.
//
// numLevels <= 0 means "as many as the mod declared": the engine does not know how deep a game goes, only the
// mod does. A caller that DOES know its own depth passes it and gets the stricter check.
func LoadLevels(recs []recfile.Record, numLevels int) ([]Level, error) {
	if numLevels <= 0 {
		for _, r := range recs {
			if dlvl, err := strconv.Atoi(strings.TrimSpace(r.ToMap("").GetStringOrDefault("dlvl", ""))); err == nil {
				numLevels = max(numLevels, dlvl+1)
			}
		}
		if numLevels <= 0 {
			return nil, fmt.Errorf("levels.rec: declares no levels")
		}
	}
	levels := make([]Level, numLevels)
	for _, r := range recs {
		if _, side := Field(r, "map"); side {
			continue // LoadSideLevels' business
		}
		dlvl, err := strconv.Atoi(strings.TrimSpace(r.ToMap("").GetStringOrDefault("dlvl", "")))
		if err != nil {
			return nil, fmt.Errorf("levels.rec: dlvl: %w", err)
		}
		if dlvl < 1 || dlvl >= numLevels {
			return nil, fmt.Errorf("levels.rec: dlvl %d out of range", dlvl)
		}
		l, err := parseLevel(r)
		if err != nil {
			return nil, err
		}
		levels[dlvl] = l
	}
	for dlvl := 1; dlvl < numLevels; dlvl++ {
		if levels[dlvl].Generator == "" {
			return levels, fmt.Errorf("levels.rec: no generator for dungeon level %d", dlvl)
		}
	}
	return levels, nil
}

// LoadSideLevels reads the records keyed by "map" instead of "dlvl": levels off the main descent, by map name.
func LoadSideLevels(recs []recfile.Record) (map[string]Level, error) {
	side := map[string]Level{}
	for _, r := range recs {
		name, ok := Field(r, "map")
		if !ok {
			continue
		}
		l, err := parseLevel(r)
		if err != nil {
			return nil, err
		}
		depth, err := strconv.Atoi(strings.TrimSpace(r.ToMap("").GetStringOrDefault("depth", "")))
		if err != nil || l.UpTarget == nil {
			return nil, fmt.Errorf("levels.rec: side level %q wants a depth and an up", name)
		}
		l.Map, l.Depth = name, depth
		side[name] = l
	}
	return side, nil
}

func parseLevel(r recfile.Record) (Level, error) {
	name := strings.TrimSpace(r.ToMap("").GetStringOrDefault("generator", ""))
	s, ok := Lookup(name)
	if !ok {
		return Level{}, fmt.Errorf("levels.rec: unknown generator %q (have %s)", name, strings.Join(Names(), ", "))
	}
	cfg, err := s.Parse(r)
	if err != nil {
		return Level{}, err
	}
	l := Level{Generator: name, Config: cfg}
	l.Name, _ = Field(r, "name")
	l.Name = strings.ReplaceAll(l.Name, `\n`, "\n") // a level's name is drawn over two lines
	l.Tiles, _ = Field(r, "tiles")
	for _, t := range FieldsNamed(r, "transition") {
		tr, err := parseTransition(t)
		if err != nil {
			return Level{}, err
		}
		l.Transitions = append(l.Transitions, tr)
	}
	if up, ok := Field(r, "up"); ok {
		p := strings.Split(up, ",")
		if len(p) != 2 {
			return Level{}, fmt.Errorf("levels.rec: up %q: want target map, target location", up)
		}
		l.UpTarget = &Transition{TargetMap: strings.TrimSpace(p[0]), TargetLocation: strings.TrimSpace(p[1]), Up: true}
	}
	return l, nil
}

// parseTransition reads "location name, target map, target location, up|down".
func parseTransition(s string) (Transition, error) {
	p := strings.Split(s, ",")
	if len(p) != 4 {
		return Transition{}, fmt.Errorf("levels.rec: transition %q: want name, target map, target location, up|down", s)
	}
	dir := strings.TrimSpace(p[3])
	if dir != "up" && dir != "down" {
		return Transition{}, fmt.Errorf("levels.rec: transition %q: direction is %q, want up or down", s, dir)
	}
	return Transition{LocationName: strings.TrimSpace(p[0]), TargetMap: strings.TrimSpace(p[1]),
		TargetLocation: strings.TrimSpace(p[2]), Up: dir == "up"}, nil
}

// Field is the first value a record carries under key. Generators parsing their own knobs share it rather than
// each keeping a copy.
func Field(r recfile.Record, key string) (string, bool) {
	for _, f := range r {
		if f.Name == key {
			return strings.TrimSpace(f.Value), true
		}
	}
	return "", false
}

// FieldsNamed returns every value a record carries under key, in file order: a knob a mod may repeat.
func FieldsNamed(r recfile.Record, key string) []string {
	var out []string
	for _, f := range r {
		if f.Name == key {
			out = append(out, strings.TrimSpace(f.Value))
		}
	}
	return out
}

// Int parses one integer knob, naming the field when it does not.
func Int(r recfile.Record, key string, def int) (int, error) {
	v, ok := Field(r, key)
	if !ok {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def, fmt.Errorf("levels.rec: %s %q: %w", key, v, err)
	}
	return n, nil
}
