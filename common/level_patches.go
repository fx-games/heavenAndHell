package common

import (
	"fmt"
	"strconv"
	"strings"

	"rpg/dungeon"
	"rpg/gridmap"
)

// A level's patches (levels.rec's "patch:", LEVELS.md) are D1's ObjChangeMap as data: megatiles x1..x2, y1..y2
// of a built D1 level switch to the level's alternate dungeon (pdungeon). A region is one of
//
//	<x1> <y1> <x2> <y2>        fixed megatiles; each a number, or the quest set piece's left, top, right,
//	                           bottom, midx or midy, with an optional +n / -n
//	used <OBJ_...> ...         every used object of these kinds opens the range its generator gave it
//	group <OBJ_...> ...        every object of these kinds opens its range once its whole lever group is used
//	range <OBJ_...> ...        every object of these kinds opens its range, used or not (0 0 0 0 is none)
//
// A patch without apply_when always holds. Nothing is stored: every patch follows from the quest vars and the
// objects' state, which the save keeps, so a level applies its patches again whenever it is built, loaded or
// its quests change. ChangeMap is idempotent, so applying one twice changes nothing.
type d1Region struct {
	fixed [4]d1Term
	kinds map[int]bool // used / group / range: the object kinds
	form  string       // "used", "group" or "range"
}

// d1Term is one fixed coordinate: base (a set piece edge, "" for none) plus n.
type d1Term struct {
	base string
	n    int
}

var d1RegionBases = map[string]bool{"left": true, "top": true, "right": true, "bottom": true, "midx": true, "midy": true}

// parseD1Region reads a patch's region; objectKind resolves an OBJ_ name to its kind.
func parseD1Region(region string, objectKind func(string) (int, bool)) (d1Region, error) {
	f := strings.Fields(region)
	if len(f) > 1 && (f[0] == "used" || f[0] == "group" || f[0] == "range") {
		r := d1Region{kinds: map[int]bool{}, form: f[0]}
		for _, name := range f[1:] {
			k, ok := objectKind(name)
			if !ok {
				return r, fmt.Errorf("unknown object %q", name)
			}
			r.kinds[k] = true
		}
		return r, nil
	}
	if len(f) != 4 {
		return d1Region{}, fmt.Errorf("region %q: want x1 y1 x2 y2, or used, group or range <objects>", region)
	}
	var r d1Region
	for i, s := range f {
		base, off := s, "0"
		if j := strings.IndexAny(s, "+-"); j > 0 {
			base, off = s[:j], s[j:]
		}
		n, err := strconv.Atoi(strings.TrimPrefix(off, "+"))
		if v, e := strconv.Atoi(s); e == nil {
			r.fixed[i] = d1Term{n: v}
			continue
		}
		if err != nil || !d1RegionBases[base] {
			return r, fmt.Errorf("region %q: %q is not a number or left/top/right/bottom/midx/midy[+-n]", region, s)
		}
		r.fixed[i] = d1Term{base: base, n: n}
	}
	return r, nil
}

// at is the term's megatile for set piece sp (x, y, w, h).
func (t d1Term) at(sp [4]int) int {
	return t.n + map[string]int{"": 0, "left": sp[0], "top": sp[1], "right": sp[0] + sp[2], "bottom": sp[1] + sp[3],
		"midx": sp[2]>>1 + sp[0], "midy": sp[3]>>1 + sp[1]}[t.base]
}

// d1ObjectKind resolves D1's own object name (xl_obj.txt's OBJ_...) to its kind.
func (f *Factory) d1ObjectKind(name string) (int, bool) {
	if f == nil || f.Generator == nil {
		return 0, false
	}
	for k, o := range f.Generator.D.Objects {
		if o.Name == name {
			return k, true
		}
	}
	return 0, false
}

// checkPatches is LoadLevels' check of a level's patches: a region D1 can read, a condition that compiles.
func (f *Factory) checkPatches(l dungeon.Level) error {
	for _, p := range l.Patches {
		if _, err := parseD1Region(p.Region, f.d1ObjectKind); err != nil {
			return fmt.Errorf("levels.rec: %s: patch %s: %w", l.Map, p.Name, err)
		}
		if _, err := compileQuestCondition(p.ApplyWhen); p.ApplyWhen != "" && err != nil {
			return fmt.Errorf("levels.rec: %s: patch %s: apply_when: %w", l.Map, p.Name, err)
		}
	}
	return nil
}

// applyPatches applies every patch of m's level whose condition holds, in file order.
func (s *State) applyPatches(m *gridmap.GridMap[*Actor, *Item, Object]) {
	gen := s.mapGen
	lvl := gen.d1Levels[m.GetName()]
	if lvl == nil {
		return
	}
	sp := gen.setPiece(m)
	for _, p := range s.Level(m.GetName()).Patches {
		if p.ApplyWhen != "" && !s.factory.questCondition(p.ApplyWhen) {
			continue
		}
		r, err := parseD1Region(p.Region, s.factory.d1ObjectKind)
		if err != nil {
			panic(err) // checked at load
		}
		if r.kinds == nil {
			gen.changeD1Map(m, r.fixed[0].at(sp), r.fixed[1].at(sp), r.fixed[2].at(sp), r.fixed[3].at(sp))
		} else {
			member := func(kind int) bool { return r.kinds[kind] }
			for _, obj := range m.Objects() {
				o, ok := obj.(*D1Object)
				if !ok || !r.kinds[o.O.Kind] {
					continue
				}
				v := o.O
				switch {
				case r.form == "range" && v.Var1 == 0 && v.Var2 == 0 && v.Var3 == 0 && v.Var4 == 0:
				case r.form == "range", r.form == "used" && !v.Active, r.form == "group" && d1ObjectGroupUsed(m, member, v.Var8):
					gen.changeD1Map(m, o.O.Var1, o.O.Var2, o.O.Var3, o.O.Var4)
				}
			}
		}
		if p.Objects != "" { // e.g. Blood2.DUN's torches lighting the hall (OperatePedistal)
			for _, o := range s.factory.D1().D.DunObjects(p.Objects, 2*sp[0], 2*sp[1]) {
				d1PlaceObject(s.factory, m, lvl, o, toCell(o.X, o.Y))
			}
			delete(s.glowCache, m)
		}
	}
}

// QuestsChanged brings m up to date after its quests or objects changed: exits that opened, patches that apply.
func (s *State) QuestsChanged(m *gridmap.GridMap[*Actor, *Item, Object]) {
	if m == nil {
		return
	}
	s.wireExits(m)
	s.applyPatches(m)
}

// Arrive is what arriving on a level does beyond the transition itself: the level's on_enter_level triggers
// (PlayDungMsgs, ResyncQuests' items), then QuestsChanged. A town has only the triggers.
func Arrive(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	if m == nil {
		return
	}
	if s.Level(m.GetName()).Town {
		s.factory.fireQuestTriggers(EventEnterLevel, m.GetName(), "")
		return
	}
	clear(d1GossipSeeds) // the town's towners are rolled anew on the way back
	s.factory.fireQuestTriggers(EventEnterLevel, m.GetName(), "")
	s.QuestsChanged(m)
}
