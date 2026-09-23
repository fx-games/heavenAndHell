package rules

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"rpg/src/recfile"
	"rpg/src/util"
)

// The tables that used to be Go literals live in this mod's own .rec files. ReadRec and readRecMulti are the only
// two ways they are read: a table group gets a function that turns records into whatever the literal used to be,
// and nothing more. Fields are looked up by name, so a .rec may carry columns this game ignores.
func ReadRec(fsys fs.FS, name string) ([]recfile.Record, error) {
	recs, err := readRecMulti(fsys, name)
	if err != nil {
		return nil, err
	}
	return recs["default"], nil
}

func readRecMulti(fsys fs.FS, name string) (map[string][]recfile.Record, error) {
	f, err := fsys.Open(DataPath(name))
	if err != nil {
		return nil, fmt.Errorf("d1 data %s: %w", name, err)
	}
	defer f.Close()
	return recfile.ReadMulti(f), nil
}

func loadMonsterTables(fsys fs.FS, d *Data) error {
	recs, err := readRecMulti(fsys, "monsters.rec")
	// monsters.rec glosses the AI_* brains in plain English for the actor popup. A mod whose monsters are driven
	// by skills rather than by Diablo's brains names no AI_* at all and ships no file; the glossary stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		d.MonsterAIs = map[string]MonsterAI{}
		return nil
	}
	if err != nil {
		return err
	}
	d.MonsterAIs = make(map[string]MonsterAI, len(recs["ai"]))
	for _, r := range recs["ai"] {
		m := r.ToMap("")
		id := m.GetStringOrDefault("id", "")
		if _, dup := d.MonsterAIs[id]; dup {
			return fmt.Errorf("monsters.rec: ai %q appears twice", id)
		}
		d.MonsterAIs[id] = MonsterAI{Description: m.GetStringOrDefault("description", ""),
			Talker: m.GetBoolOrFalse("talker"), NeedsQuest: m.GetBoolOrFalse("needs_quest"),
			FightsWhen: m.GetStringOrDefault("fights_when", ""), Ranged: m.GetBoolOrFalse("ranged"),
			Pack: m.GetBoolOrFalse("pack"), Dormant: m.GetBoolOrFalse("dormant"), Evasive: m.GetBoolOrFalse("evasive"),
			ArchBoss: m.GetBoolOrFalse("arch_boss")}
	}
	return nil
}

// MonsterAI is one AI_* brain's record in monsters.rec. The brain itself is code (game/ai.go's think()).
type MonsterAI struct {
	Description string // plain-English gloss, shown in the actor description popup
	Talker      bool   // M_Talker: a quest monster that speaks before it fights
	NeedsQuest  bool   // only fights once its quest has set it up; never picked by the random-unique debug spawn
	FightsWhen  string // a talker's condition (quests.rec's language) for fighting this turn; "": done talking
	Ranged      bool   // M_Ranged: goes after a golem anywhere, not only next to it
	Pack        bool   // a fallen one: answers a howl, flees when one of its pack dies (M_FallenFear)
	Dormant     bool   // starts as a statue; the first blow only wakes it (the gargoyle)
	Evasive     bool   // can't be hit while off its normal goal (the counselors)
	ArchBoss    bool   // Diablo: holy bolt hurts it, and stone, doppelgangers and berserk leave it be
}

// fieldsNamed returns every value a record carries under key, in file order. Tables that are really one long list
// of numbers (D1 has a few) spread it over repeated fields so the file stays readable and diffs stay small.
func fieldsNamed(r recfile.Record, key string) []string {
	var out []string
	for _, f := range r {
		if f.Name == key {
			out = append(out, f.Value)
		}
	}
	return out
}

// readAffixes reads affixes.rec (the fields are documented there); a mod without one rolls its items plain.
func readAffixes(fsys fs.FS) (prefixes, suffixes []Affix, err error) {
	recs, err := ReadRec(fsys, "affixes.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	on := map[string]int{"weapon": PLTWeap, "bow": PLTBow, "staff": PLTStaff, "armor": PLTArmo, "shield": PLTShld, "jewelry": PLTMisc}
	alignment := map[string]int{"neutral": 0, "cursed": 1, "good": 16}
	for _, r := range recs {
		a, slot := Affix{Ok: true}, ""
		for _, f := range r {
			switch f.Name {
			case "internal_name":
				a.ID = f.Value
			case "name":
				a.Name = f.Value
			case "slot":
				slot = f.Value
			case "power":
				a.Power = f.Value
			case "values":
				v := util.NewIntervalFromString(f.Value)
				a.P1, a.P2 = v.Min(), v.Max()
			case "min_level":
				a.MinLvl = f.AsInt()
			case "on":
				for _, w := range strings.Fields(f.Value) {
					bit, ok := on[w]
					if !ok {
						return nil, nil, fmt.Errorf("affixes.rec %s: on %q: want weapon bow staff armor shield jewelry", a.ID, w)
					}
					a.IType |= bit
				}
			case "alignment":
				g, ok := alignment[f.Value]
				if !ok {
					return nil, nil, fmt.Errorf("affixes.rec %s: alignment %q: want neutral, cursed or good", a.ID, f.Value)
				}
				a.GOE = g
			case "double":
				a.Double = f.AsBool()
			case "in_good_rolls":
				a.Ok = f.AsBool()
			case "price":
				v := util.NewIntervalFromString(f.Value)
				a.MinVal, a.MaxVal = v.Min(), v.Max()
			case "price_multiplier":
				a.MultVal = f.AsInt()
			}
		}
		switch slot {
		case "prefix":
			prefixes = append(prefixes, a)
		case "suffix":
			suffixes = append(suffixes, a)
		default:
			return nil, nil, fmt.Errorf("affixes.rec %s: slot %q: want prefix or suffix", a.ID, slot)
		}
	}
	return prefixes, suffixes, nil
}

// readUniqueItems reads unique_items.rec; a mod without one has no unique items.
func readUniqueItems(fsys fs.FS) ([]UniqueItemData, error) {
	recs, err := ReadRec(fsys, "unique_items.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []UniqueItemData
	for _, r := range recs {
		u := UniqueItemData{}
		for _, f := range r {
			switch f.Name {
			case "internal_name":
				u.ID = f.Value
			case "name":
				u.Name = f.Value
			case "base":
				u.Base = f.Value
			case "min_level":
				u.MinLvl = f.AsInt()
			case "value":
				u.Value = f.AsInt()
			case "power":
				if u.NumPL == len(u.Powers) {
					return nil, fmt.Errorf("unique_items.rec %s: more than %d powers", u.ID, len(u.Powers))
				}
				var p Power
				if _, err := fmt.Sscan(f.Value, &p.Power, &p.P1, &p.P2); err != nil {
					return nil, fmt.Errorf("unique_items.rec %s: power %q: %w", u.ID, f.Value, err)
				}
				u.Powers[u.NumPL] = p
				u.NumPL++
			}
		}
		out = append(out, u)
	}
	return out, nil
}

// readRollOrders reads roll_order.rec: per table, the ids in the order D1 walks it when rolling. A mod without
// the file lists none.
func readRollOrders(fsys fs.FS) (map[string][]string, error) {
	recs, err := ReadRec(fsys, "roll_order.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, r := range recs {
		table := r.ToMap("").GetStringOrDefault("table", "")
		if _, dup := out[table]; dup {
			return nil, fmt.Errorf("roll_order.rec: table %q appears twice", table)
		}
		out[table] = fieldsNamed(r, "id")
	}
	return out, nil
}

// sortByRollOrder puts s into the order listed for it in roll_order.rec, then everything the list leaves out by
// id, so no roll depends on where a file happens to put a record.
func sortByRollOrder[T any](s []T, list []string, id func(T) string) error {
	pos := make(map[string]int, len(list))
	for i, x := range list {
		if _, dup := pos[x]; dup {
			return fmt.Errorf("roll_order.rec lists %q twice", x)
		}
		pos[x] = i
	}
	found := 0
	for _, r := range s {
		if _, ok := pos[id(r)]; ok {
			found++
		}
	}
	if found != len(list) {
		return fmt.Errorf("roll_order.rec lists %d ids its table doesn't have", len(list)-found)
	}
	slices.SortFunc(s, func(a, b T) int {
		pa, oka := pos[id(a)]
		pb, okb := pos[id(b)]
		switch {
		case oka && okb:
			return cmp.Compare(pa, pb)
		case oka != okb:
			if oka {
				return -1
			}
			return 1
		}
		return cmp.Compare(id(a), id(b))
	})
	return nil
}
