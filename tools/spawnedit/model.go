package main

import (
	"fmt"
	"math/rand"
	"os"
	"rpg/files"
	"rpg/src/dungen"
	"rpg/src/dungeon"
	"rpg/src/hhm"
	"rpg/src/recfile"
	"slices"
	"strconv"
	"strings"
)

// enemy is one randomly rolled monster: the four enemies.rec fields this tool edits.
type enemy struct {
	id, name        string
	icon            int32
	min, max, avail int
	cost            int  // spawn_cost
	unique          bool // placed by its room, never rolled: shown, not edited
	danger          int  // the experience it pays: the list sorts by it
}

// level is one non-town levels.rec record.
type level struct {
	id, name, tiles string
	depth           int
	mons            []string // a side level's monster lines; a regular level has none and rolls by dlvl
	generator       string
	prefix          string   // template_prefix: the rooms it may grow from
	require         []string // require_room
	rec             recfile.Record
}

// grows reports whether room is one of l's templates: accretion levels grow rooms named with their prefix.
func (l *level) grows(room string) bool {
	return l.generator == "accretion" && strings.HasPrefix(room, l.prefix)
}

// moveRoom makes room required on to and on no other level; the game forbids it everywhere else by itself.
func moveRoom(levels []*level, room string, to *level) {
	for _, l := range levels {
		l.require = slices.DeleteFunc(l.require, func(r string) bool { return r == room })
	}
	to.require = append(to.require, room)
}

// placeable grows l once with require as its required rooms: an error when the rooms never fit.
func placeable(l *level, require []string) error {
	s, _ := dungeon.Lookup(l.generator)
	cfg, err := s.Parse(l.rec)
	if err != nil {
		return err
	}
	c := cfg.(dungen.AccretionConfig)
	c.Require = require
	_, err = dungen.Build(c, rand.New(rand.NewSource(1)), files.GetFileSystem())
	return err
}

// spawn is a quests.rec trigger that spawns uniques when the hero enters a level: moving one of them moves the
// trigger, and so every monster it spawns.
type spawn struct {
	marker   string // its first spawn_monster line, which finds the record again
	from, to string // the level it fires on, as read and as edited
	quests   []string
}

// questSpawns finds the on_enter_level triggers that spawn_monster, by the monster they spawn. A quest whose
// level: is the trigger's level and whose key the trigger names moves along.
func questSpawns(recs []recfile.Record) map[string]*spawn {
	out := map[string]*spawn{}
	for _, r := range recs {
		ev, _ := dungeon.Field(r, "event")
		if ev != "on_enter_level" {
			continue
		}
		subject, _ := dungeon.Field(r, "subject")
		when, _ := dungeon.Field(r, "when")
		var sp *spawn
		for _, do := range dungeon.FieldsNamed(r, "do") {
			do = strings.TrimSpace(do)
			id, ok := strings.CutPrefix(do, "spawn_monster(")
			if !ok {
				continue
			}
			if sp == nil {
				sp = &spawn{marker: "do: " + do, from: subject, to: subject}
				for _, q := range recs {
					key, _ := dungeon.Field(q, "key")
					lvl, _ := dungeon.Field(q, "level")
					if key != "" && lvl == subject && strings.Contains(when, `"`+key+`"`) {
						sp.quests = append(sp.quests, key)
					}
				}
			}
			out[strings.TrimSpace(strings.Split(id, ",")[0])] = sp
		}
	}
	return out
}

// setSubject rewrites the subject: line of the record holding the line marker.
func setSubject(text, marker, subject string) string {
	lines := strings.Split(text, "\n")
	at := slices.Index(lines, marker)
	if at < 0 {
		return text
	}
	for i := at; i >= 0 && strings.TrimSpace(lines[i]) != ""; i-- {
		if strings.HasPrefix(lines[i], "subject:") {
			lines[i] = "subject: " + subject
		}
	}
	return strings.Join(lines, "\n")
}

// roomsOf maps each unique to the room template that places it: an NPC entity with its UniqueName, in the
// template itself or in a decoration the template uses (UsesDecoration is a name prefix).
func roomsOf(p *hhm.Project) map[string]string {
	out := map[string]string{}
	for _, t := range p.Levels {
		if t.Props.IsDecoration {
			continue
		}
		for _, d := range p.Levels {
			if d != t && !(d.Props.IsDecoration && t.Props.UsesDecoration != "" && strings.HasPrefix(d.Name, t.Props.UsesDecoration)) {
				continue
			}
			for _, e := range d.Entities {
				if u := e.F["UniqueName"]; u != "" {
					out[u] = t.Name
				}
			}
		}
	}
	return out
}

func (l *level) side() bool { return len(l.mons) > 0 }

// setOn puts id on l's list, or takes it off.
func (l *level) setOn(id string, on bool) {
	l.mons = slices.DeleteFunc(l.mons, func(m string) bool { return m == id })
	if on {
		l.mons = append(l.mons, id)
	}
}

// span is the first and last depth e spawns on; ok false when it never spawns at random.
func (e *enemy) span(deepest int) (lo, hi int, ok bool) {
	if e.avail == 0 {
		return 0, 0, false
	}
	hi = e.max
	if hi == 0 {
		hi = deepest
	}
	return e.min, hi, e.min <= hi
}

func (e *enemy) spawnsAt(d, deepest int) bool {
	lo, hi, ok := e.span(deepest)
	return ok && d >= lo && d <= hi
}

// add stretches e's span to take in depth d; spans are contiguous, so the depths between come along.
func (e *enemy) add(d, deepest int) {
	lo, hi, ok := e.span(deepest)
	switch {
	case !ok:
		e.min, e.max, e.avail = d, d, 1
	case d < lo:
		e.min = d
	case d > hi:
		e.max = d
	}
}

// remove takes depth d off an end of e's span; the middle can't go, the fields only hold a range.
func (e *enemy) remove(d, deepest int) error {
	lo, hi, ok := e.span(deepest)
	switch {
	case !ok || d < lo || d > hi:
		return nil
	case lo == hi:
		e.avail = 0
	case d == lo:
		e.min = d + 1
	case d == hi:
		e.max = d - 1
	default:
		return fmt.Errorf("%s spawns on %d..%d: take depths off an end, the range can't have holes", e.name, lo, hi)
	}
	return nil
}

// setList replaces every "<name>:" line of the record whose key line is "<key>: <id>" with one line per value,
// after the record's last line.
func setList(text, key, id, name string, values []string) string {
	lines := strings.Split(text, "\n")
	at := slices.Index(lines, key+": "+id)
	if at < 0 {
		return text
	}
	start := at
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	end := start
	var kept []string
	for ; end < len(lines) && strings.TrimSpace(lines[end]) != ""; end++ {
		if !strings.HasPrefix(lines[end], name+":") {
			kept = append(kept, lines[end])
		}
	}
	for _, v := range values {
		kept = append(kept, name+": "+v)
	}
	return strings.Join(slices.Concat(lines[:start], kept, lines[end:]), "\n")
}

// setFields rewrites a rec file's record whose key line is "<key>: <id>", keeping every comment and every other
// line as it is. A value equal to its default is removed, or left out when the record never had it.
func setFields(text, key, id string, fields [][3]string) string {
	lines := strings.Split(text, "\n")
	at := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == key+": "+id {
			at = i
			break
		}
	}
	if at < 0 {
		return text
	}
	end := at
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		end++
	}
	start := at
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	for _, f := range fields {
		name, val, def := f[0], f[1], f[2]
		found := -1
		for i := start; i < end; i++ {
			if strings.HasPrefix(lines[i], name+":") {
				found = i
				break
			}
		}
		switch {
		case found >= 0 && val == def:
			lines = append(lines[:found], lines[found+1:]...)
			end--
		case found >= 0:
			lines[found] = name + ": " + val
		case val != def:
			lines = append(lines[:end], append([]string{name + ": " + val}, lines[end:]...)...)
			end++
		}
	}
	return strings.Join(lines, "\n")
}

func save(enemiesPath, levelsPath, questsPath string, enemies []*enemy, levels []*level, spawns map[string]*spawn) error {
	if err := saveSpawns(questsPath, spawns); err != nil {
		return err
	}
	b, err := os.ReadFile(enemiesPath)
	if err != nil {
		return err
	}
	s := string(b)
	for _, e := range enemies {
		s = setFields(s, "internal_name", e.id, [][3]string{
			{"dlvl", strconv.Itoa(e.min), "0"}, {"max_dlvl", strconv.Itoa(e.max), "0"},
			{"random_spawn", strconv.FormatBool(e.avail != 0), "true"}, {"spawn_cost", strconv.Itoa(e.cost), "0"}})
	}
	if err := os.WriteFile(enemiesPath, []byte(s), 0o644); err != nil {
		return err
	}
	if b, err = os.ReadFile(levelsPath); err != nil {
		return err
	}
	s = string(b)
	for _, l := range levels {
		s = setList(s, "map", l.id, "monster", l.mons)
		if l.generator == "accretion" {
			s = setList(s, "map", l.id, "require_room", l.require)
		}
	}
	return os.WriteFile(levelsPath, []byte(s), 0o644)
}

func saveSpawns(path string, spawns map[string]*spawn) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)
	for _, sp := range spawns {
		if sp.to == sp.from {
			continue
		}
		s = setSubject(s, sp.marker, sp.to)
		for _, q := range sp.quests {
			s = setFields(s, "key", q, [][3]string{{"level", sp.to, ""}})
		}
		sp.from = sp.to
	}
	return os.WriteFile(path, []byte(s), 0o644)
}
