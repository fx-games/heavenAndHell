package main

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"rpg/src/dungeon"
)

// enemy is one randomly rolled monster: the four enemies.rec fields this tool edits.
type enemy struct {
	id, name             string
	icon                 int32
	min, max, avail, wgt int
	ood                  float64 // out_of_depth_chance: odds to turn up above min, from depth oodMin on
	oodMin               int
	unique               bool // placed by its room, never rolled: shown, not edited
	danger               int  // the experience it pays: the list sorts by it
}

// oodAt reports whether e can turn up at depth d out of depth: above its own dlvl, from min_level_for_ood on.
func (e *enemy) oodAt(d int) bool {
	return !d1Rules && !e.unique && e.avail != 0 && e.ood > 0 && d < e.min && d >= e.oodMin
}

// level is one non-town levels.rec record.
type level struct {
	id, name, tiles string
	depth, pct      int
	mons            []dungeon.LevelMonster // its monster lines; d1 levels keep none, 1.09 rolls by dlvl
}

// weight is how many shares of l's rolls e gets, 0 when it isn't on the list.
func (l *level) weight(id string) int {
	for _, m := range l.mons {
		if m.ID == id {
			return m.Weight
		}
	}
	return 0
}

// setWeight puts id on l's list at w shares, or takes it off at 0.
func (l *level) setWeight(id string, w int) {
	i := slices.IndexFunc(l.mons, func(m dungeon.LevelMonster) bool { return m.ID == id })
	switch {
	case i < 0 && w > 0:
		l.mons = append(l.mons, dungeon.LevelMonster{ID: id, Weight: w})
	case i >= 0 && w > 0:
		l.mons[i].Weight = w
	case i >= 0:
		l.mons = slices.Delete(l.mons, i, i+1)
	}
}

// d1Rules: D1's GetLevelMTypes reads dlvl/max_dlvl on a 0-30 scale (15*x/30+1), dungen reads them as depths.
var d1Rules bool

// span is the first and last depth e spawns on; ok false when it never spawns at random.
func (e *enemy) span(deepest int) (lo, hi int, ok bool) {
	if e.avail == 0 {
		return 0, 0, false
	}
	if d1Rules {
		return 15*e.min/30 + 1, 15*e.max/30 + 1, true
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

// rawLo/rawHi turn a depth into the dlvl / max_dlvl value that starts or ends a span there.
func rawLo(d int) int {
	if d1Rules {
		return 2 * (d - 1)
	}
	return d
}

func rawHi(d int) int {
	if d1Rules {
		return 2*(d-1) + 1
	}
	return d
}

// add stretches e's span to take in depth d; spans are contiguous, so the depths between come along.
func (e *enemy) add(d, deepest int) {
	lo, hi, ok := e.span(deepest)
	switch {
	case !ok:
		e.min, e.max, e.avail = rawLo(d), rawHi(d), 1
	case d < lo:
		e.min = rawLo(d)
	case d > hi:
		e.max = rawHi(d)
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
		e.min = rawLo(d + 1)
	case d == hi:
		e.max = rawHi(d - 1)
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

func save(enemiesPath, levelsPath string, enemies []*enemy, levels []*level) error {
	b, err := os.ReadFile(enemiesPath)
	if err != nil {
		return err
	}
	s := string(b)
	for _, e := range enemies {
		s = setFields(s, "internal_name", e.id, [][3]string{
			{"dlvl", strconv.Itoa(e.min), "0"}, {"max_dlvl", strconv.Itoa(e.max), "0"},
			{"available", strconv.Itoa(e.avail), "0"}, {"out_of_depth_chance", strconv.FormatFloat(e.ood, 'g', -1, 64), "0"}})
	}
	if err := os.WriteFile(enemiesPath, []byte(s), 0o644); err != nil {
		return err
	}
	if b, err = os.ReadFile(levelsPath); err != nil {
		return err
	}
	s = string(b)
	for _, l := range levels {
		s = setFields(s, "map", l.id, [][3]string{{"enemy_percent", strconv.Itoa(l.pct), "0"}})
		if !d1Rules {
			var lines []string
			for _, m := range l.mons {
				lines = append(lines, fmt.Sprintf("%s %d", m.ID, m.Weight))
			}
			s = setList(s, "map", l.id, "monster", lines)
		}
	}
	return os.WriteFile(levelsPath, []byte(s), 0o644)
}
