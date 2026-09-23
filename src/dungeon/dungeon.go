// Package dungeon is a mod's level table (levels.rec, docs/LEVELS.md): which levels exist, how they connect, and
// which generator builds each with which knobs. The generators themselves are strategies in package common
// (LevelStrategy), which hands LoadLevels their parsing; nothing here knows which mod is loaded.
package dungeon

import (
	"fmt"
	"strconv"
	"strings"

	"rpg/src/recfile"
)

// Config is one level's parsed generator configuration; only the strategy that parsed it looks inside.
type Config any

// Parser reads a generator's knobs from its levels.rec record. Fields the record leaves out take the
// generator's defaults, so a record naming only a generator is valid.
type Parser interface {
	Parse(recfile.Record) (Config, error)
}

// Exit is one way off a level: stepping on the named location takes you to another level's named location.
// OpenWhen, in the quest trigger language, keeps it shut until true; Tile is what the spot looks like once open.
// Declared as "exit: <location> -> <map id>, <arrival location>", optionally followed by "open_when:" and
// "exit_tile:" lines that belong to it.
type Exit struct {
	Location, Target, Arrival string
	OpenWhen, Tile            string
}

// Patch is a region of a level that changes after it is built (D1's ObjChangeMap: a lever, a book, a quest step
// opens part of the map). Declared as "patch: <name> = <region>", optionally followed by "apply_when:" (the quest
// trigger language; it applies once that holds). The region is the level family's to read; the engine keeps it
// as written.
type Patch struct {
	Name, Region, ApplyWhen string
}

// Level is one level as a mod declared it (docs/LEVELS.md): a town, a dungeon depth, a quest or side level alike.
// Config is its generator's own business; the rest is the engine's.
type Level struct {
	Map, Name   string // Map is the id every other file refers to; never parsed
	Depth       int    // difficulty and progress only; several levels may share one
	Generator   string
	Config      Config
	Tiles       string
	Music       string // a music.rec event; empty: no music
	Glyphs      string // an 8x8 glyph atlas in the mod folder: the level is drawn as text with it; empty: tiles
	Town, Start bool
	Monsters    []string // a side level's monster lines: who it rolls, instead of every record whose dlvl..max_dlvl fits
	Exits       []Exit
	Locations   map[string][2]int // named cells the level declares ("location: <name> <x> <y>"), for its quest logic
	Patches     []Patch           // in file order, which is the order they apply
}

// commonFields are the fields every level record may carry; a generator may add the ones its Parse reads.
var commonFields = map[string]bool{"map": true, "name": true, "depth": true, "generator": true, "tiles": true,
	"music": true, "glyphs": true, "town": true, "start": true, "exit": true, "open_when": true, "exit_tile": true,
	"patch": true, "location": true, "apply_when": true, "monster": true}

// LoadLevels reads a mod's whole levels.rec and checks it: every record a level with the required fields, known
// fields only (common ones plus what its generator reads), unique ids, one start, and every exit leading to a
// declared level. Called after mod selection, never at package init.
func LoadLevels(recs []recfile.Record, parsers func(generator string) (Parser, bool)) ([]Level, error) {
	var levels []Level
	ids := map[string]bool{}
	starts := 0
	for _, r := range recs {
		l, err := parseLevel(r, parsers)
		if err != nil {
			return nil, err
		}
		if ids[l.Map] {
			return nil, fmt.Errorf("levels.rec: map %q declared twice", l.Map)
		}
		ids[l.Map] = true
		if l.Start {
			starts++
		}
		levels = append(levels, l)
	}
	if starts != 1 {
		return nil, fmt.Errorf("levels.rec: %d levels say start: true, want exactly one", starts)
	}
	for _, l := range levels {
		for _, e := range l.Exits {
			if !ids[e.Target] {
				return nil, fmt.Errorf("levels.rec: %s: exit %s leads to undeclared level %q", l.Map, e.Location, e.Target)
			}
		}
	}
	return levels, nil
}

func parseLevel(r recfile.Record, parsers func(string) (Parser, bool)) (Level, error) {
	id, _ := Field(r, "map")
	fail := func(format string, a ...any) (Level, error) {
		return Level{}, fmt.Errorf("levels.rec: %s: %s", id, fmt.Sprintf(format, a...))
	}
	for _, need := range []string{"map", "depth", "generator", "tiles"} {
		if v, ok := Field(r, need); !ok || v == "" {
			return fail("missing %s", need)
		}
	}
	gen, _ := Field(r, "generator")
	s, ok := parsers(gen)
	if !ok {
		return fail("unknown generator %q", gen)
	}
	read = map[string]bool{}
	cfg, err := s.Parse(r)
	accepted := read
	read = nil
	if err != nil {
		return Level{}, err
	}
	for _, f := range r {
		if !commonFields[f.Name] && !accepted[f.Name] {
			return fail("unknown field %q for generator %s", f.Name, gen)
		}
	}
	l := Level{Map: id, Generator: gen, Config: cfg}
	if l.Depth, err = Int(r, "depth", 0); err != nil {
		return Level{}, err
	}
	l.Name, _ = Field(r, "name")
	l.Name = strings.ReplaceAll(l.Name, `\n`, "\n") // a level's name is drawn over two lines
	l.Tiles, _ = Field(r, "tiles")
	l.Music, _ = Field(r, "music")
	l.Glyphs, _ = Field(r, "glyphs")
	town, _ := Field(r, "town")
	start, _ := Field(r, "start")
	l.Town, l.Start = town == "true", start == "true"
	for _, f := range r {
		v := strings.TrimSpace(f.Value)
		switch f.Name {
		case "monster":
			if strings.ContainsAny(v, " \t") {
				return fail("monster %q: want <internal_name> (odds come from spawn_cost now)", v)
			}
			l.Monsters = append(l.Monsters, v)
		case "location":
			var name string
			var x, y int
			if n, _ := fmt.Sscanf(v, "%s %d %d", &name, &x, &y); n != 3 {
				return fail("location %q: want <name> <x> <y>", v)
			}
			if l.Locations == nil {
				l.Locations = map[string][2]int{}
			}
			l.Locations[name] = [2]int{x, y}
		case "exit":
			loc, to, ok := strings.Cut(v, "->")
			target, arrival, ok2 := strings.Cut(to, ",")
			if !ok || !ok2 {
				return fail("exit %q: want <location> -> <map id>, <arrival location>", v)
			}
			l.Exits = append(l.Exits, Exit{Location: strings.TrimSpace(loc), Target: strings.TrimSpace(target), Arrival: strings.TrimSpace(arrival)})
		case "patch":
			name, region, ok := strings.Cut(v, "=")
			if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(region) == "" {
				return fail("patch %q: want <name> = <region>", v)
			}
			for _, p := range l.Patches {
				if p.Name == strings.TrimSpace(name) {
					return fail("patch %q declared twice", p.Name)
				}
			}
			l.Patches = append(l.Patches, Patch{Name: strings.TrimSpace(name), Region: strings.TrimSpace(region)})
		case "apply_when":
			if len(l.Patches) == 0 {
				return fail("%s before any patch", f.Name)
			}
			l.Patches[len(l.Patches)-1].ApplyWhen = v
		case "open_when", "exit_tile":
			if len(l.Exits) == 0 {
				return fail("%s before any exit", f.Name)
			}
			e := &l.Exits[len(l.Exits)-1]
			if f.Name == "open_when" {
				e.OpenWhen = v
			} else {
				e.Tile = v
			}
		}
	}
	return l, nil
}

// read collects the fields a generator's Parse asks for, through Field, FieldsNamed and Int: those are the fields
// it accepts. ponytail: a field a Parse only reads on some path counts as unknown on the others.
var read map[string]bool

// Field is the first value a record carries under key. Generators parsing their own knobs share it rather than
// each keeping a copy.
func Field(r recfile.Record, key string) (string, bool) {
	if read != nil {
		read[key] = true
	}
	for _, f := range r {
		if f.Name == key {
			return strings.TrimSpace(f.Value), true
		}
	}
	return "", false
}

// FieldsNamed returns every value a record carries under key, in file order: a knob a mod may repeat.
func FieldsNamed(r recfile.Record, key string) []string {
	if read != nil {
		read[key] = true
	}
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

// Float parses one decimal knob, naming the field when it does not.
func Float(r recfile.Record, key string, def float64) (float64, error) {
	v, ok := Field(r, key)
	if !ok {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def, fmt.Errorf("levels.rec: %s %q: %w", key, v, err)
	}
	return f, nil
}
