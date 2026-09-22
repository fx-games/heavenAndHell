package d1

import (
	"errors"
	"io/fs"
)

// oil is one of Hellfire's oils; the table lives in assets/data/oils.rec and is loaded by Load.
type oil struct {
	MiscID string
	Name   string
	MinLvl int
	Value  int
}

var oils []oil

func loadOils(fsys fs.FS) ([]oil, error) {
	recs, err := readRec(fsys, "oils.rec")
	// A mod that does not have this Diablo feature ships no file for it, and an absent feature is not an
	// error: what the table would have filled stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]oil, 0, len(recs))
	for _, r := range recs {
		m := r.ToMap("")
		out = append(out, oil{
			MiscID: m.GetStringOrDefault("misc_id", ""),
			Name:   m.GetStringOrDefault("name", ""),
			MinLvl: m.GetIntOrDefault("min_lvl", 0),
			Value:  m.GetIntOrDefault("value", 0),
		})
	}
	return out, nil
}

// IsOil reports one of Hellfire's oils (IMISC_OILFIRST < id < IMISC_OILLAST), the generic "Oil" included.
func IsOil(miscID string) bool {
	if miscID == "IMISC_OILOF" {
		return true
	}
	for _, o := range oils {
		if o.MiscID == miscID {
			return true
		}
	}
	return false
}

// getOilType ports GetOilType: a plain "Oil" becomes one of the ten, picked among those the level allows.
func (g *Generator) getOilType(it *Item, maxLvl int) {
	if maxLvl == 0 {
		maxLvl = 1
	}
	var rnd []int
	for i, o := range oils {
		if o.MinLvl <= maxLvl {
			rnd = append(rnd, i)
		}
	}
	o := oils[rnd[g.random(len(rnd))]]
	it.Name, it.IName, it.MiscID = o.Name, o.Name, o.MiscID
	it.Value, it.IValue = o.Value, o.Value
}

// DurIndestructible is D1's DUR_INDESTRUCTIBLE.
const DurIndestructible = 255

// OilTarget reports whether an oil can be poured on an item at all (ApplyOilToItem's first two switches).
func OilTarget(oil string, it *Item) bool {
	switch it.Class {
	case "ICLASS_MISC", "ICLASS_GOLD", "ICLASS_QUEST", "ICLASS_NONE":
		return false
	}
	switch oil {
	case "IMISC_OILACC", "IMISC_OILMAST", "IMISC_OILSHARP":
		return it.Class != "ICLASS_ARMOR"
	case "IMISC_OILDEATH":
		return it.Class != "ICLASS_ARMOR" && it.Type != ITypeBow
	case "IMISC_OILHARD", "IMISC_OILIMP":
		return it.Class != "ICLASS_WEAPON"
	}
	return true
}

// ApplyOil ports ApplyOilToItem: it edits the item in place and reports whether the oil was used up on it.
// intn is the roll source (d1Rng.Intn), so the caller keeps the level generator's RNG out of it.
func ApplyOil(oil string, it *Item, intn func(int) int) bool {
	if !OilTarget(oil, it) {
		return false
	}
	between := func(lo, hi int) int { return lo + intn(hi-lo+1) }
	switch oil {
	case "IMISC_OILACC":
		if it.PLToHit < 50 {
			it.PLToHit += between(1, 2)
		}
	case "IMISC_OILMAST":
		if it.PLToHit < 100 {
			it.PLToHit += between(3, 5)
		}
	case "IMISC_OILSHARP":
		if it.MaxDam-it.MinDam < 30 && it.MaxDam < 255 {
			it.MaxDam++
		}
	case "IMISC_OILDEATH":
		if it.MaxDam-it.MinDam < 30 && it.MaxDam < 254 {
			it.MinDam++
			it.MaxDam += 2
		}
	case "IMISC_OILSKILL":
		r := between(5, 10)
		it.MinStr, it.MinMag, it.MinDex = max(it.MinStr-r, 0), max(it.MinMag-r, 0), max(it.MinDex-r, 0)
	case "IMISC_OILBSMTH":
		switch {
		case it.MaxDur == DurIndestructible:
		case it.Durability < it.MaxDur:
			it.Durability = min(it.Durability+(it.MaxDur+4)/5, it.MaxDur)
		case it.MaxDur < 100:
			it.MaxDur++
			it.Durability = it.MaxDur
		}
	case "IMISC_OILFORT":
		if it.MaxDur != DurIndestructible && it.MaxDur < 200 {
			r := between(10, 50)
			it.MaxDur += r
			it.Durability += r
		}
	case "IMISC_OILPERM":
		it.Durability, it.MaxDur = DurIndestructible, DurIndestructible
	case "IMISC_OILHARD":
		if it.AC < 60 {
			it.AC += between(1, 2)
		}
	case "IMISC_OILIMP":
		if it.AC < 120 {
			it.AC += between(3, 5)
		}
	default:
		return false
	}
	return true
}

// Hellfire's extra item effects (ItemSpecialEffectHf), the ones D1's ISPL_ flags have no room for.
const (
	IHfDevastation  = 1 << iota // 5% of hits deal triple damage
	IHfJesters                  // every hit scales by a wild 0-500%
	IHfPeril                    // hits deal double damage and cost the wielder some life
	IHfDecay                    // the weapon loses 5% damage per hit until it falls apart
	IHfDoppelganger             // 10% of hits split an ordinary monster in two
	IHfACVsDemons               // +40 armour class against demons
	IHfACVsUndead               // +20 armour class against the undead
	IHfManaToLife               // half the mana pool becomes life
	IHfLifeToMana               // two fifths of the life pool becomes mana
)

// JesterDamage ports the Jester's roll: 0-100% of the damage, or 100-600% when the roll goes high.
func JesterDamage(dam int, intn func(int) int) int {
	r := intn(201)
	if r >= 100 {
		r = 100 + (r-100)*5
	}
	return dam * r / 100
}
