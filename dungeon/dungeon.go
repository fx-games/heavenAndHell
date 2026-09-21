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

// Level is one dungeon level as a mod declared it: which generator builds it, and that generator's configuration.
type Level struct {
	Generator string
	Config    Config
}

// LoadLevels reads a mod's whole level list. Levels are indexed by dungeon level, so index 0 (town) stays empty
// and every level from 1 to numLevels-1 must be declared. Called after mod selection, never at package init.
func LoadLevels(recs []recfile.Record, numLevels int) ([]Level, error) {
	levels := make([]Level, numLevels)
	for _, r := range recs {
		dlvl, err := strconv.Atoi(strings.TrimSpace(r.ToMap("").GetStringOrDefault("dlvl", "")))
		if err != nil {
			return nil, fmt.Errorf("levels.rec: dlvl: %w", err)
		}
		if dlvl < 1 || dlvl >= numLevels {
			return nil, fmt.Errorf("levels.rec: dlvl %d out of range", dlvl)
		}
		name := strings.TrimSpace(r.ToMap("").GetStringOrDefault("generator", ""))
		s, ok := Lookup(name)
		if !ok {
			return nil, fmt.Errorf("levels.rec: unknown generator %q (have %s)", name, strings.Join(Names(), ", "))
		}
		cfg, err := s.Parse(r)
		if err != nil {
			return nil, err
		}
		levels[dlvl] = Level{Generator: name, Config: cfg}
	}
	for dlvl := 1; dlvl < numLevels; dlvl++ {
		if levels[dlvl].Generator == "" {
			return levels, fmt.Errorf("levels.rec: no generator for dungeon level %d", dlvl)
		}
	}
	return levels, nil
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
