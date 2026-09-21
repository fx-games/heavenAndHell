package d1

import (
	"errors"
	"fmt"
	"io/fs"
)

// Property is one of an item's magical powers: Key identifies it across items, Min/Max hold its value (equal for
// single values, 1 for flags).
type Property struct {
	Key      string
	Min, Max int
}

type propertyDef struct {
	format func(min, max int) string
	better int // +1: higher is better (or good to have), -1: lower is better (or harmful), 0: never compared
}

func ranged(format string) func(int, int) string {
	return func(lo, hi int) string { return fmt.Sprintf(format, lo, hi) }
}
func text(s string) func(int, int) string { return func(int, int) string { return s } }
func tiers(names ...string) func(int, int) string {
	return func(v, _ int) string { return names[min(max(v, 1), len(names))-1] }
}

// Damage and Armor are compared like properties but printed by the item tooltip itself.
const (
	KeyDamage = "damage"
	KeyArmor  = "armor"
)

// propertyDefs is filled by Load from assets/data/properties.rec. The wording lives there; the four ways a
// value turns into a line stay here, because they are code.
var propertyDefs map[string]propertyDef

func loadPropertyDefs(fsys fs.FS) (map[string]propertyDef, error) {
	recs, err := readRec(fsys, "properties.rec")
	// A mod that does not have this Diablo feature ships no file for it, and an absent feature is not an
	// error: what the table would have filled stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]propertyDef{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]propertyDef, len(recs))
	for _, r := range recs {
		m := r.ToMap("")
		key := m.GetStringOrDefault("key", "")
		if key == "" {
			return nil, fmt.Errorf("properties.rec: record with no key")
		}
		format, scale := m.GetStringOrDefault("format", ""), m.GetIntOrDefault("scale", 1)
		d := propertyDef{better: m.GetIntOrDefault("better", 0)}
		switch kind := m.GetStringOrDefault("kind", ""); kind {
		case "signed":
			d.format = func(v, _ int) string { return fmt.Sprintf(format, v*scale) }
		case "ranged":
			d.format = ranged(format)
		case "text":
			d.format = text(format)
		case "tiers":
			names := fieldsNamed(r, "tier")
			if len(names) == 0 {
				return nil, fmt.Errorf("properties.rec: %s: tiers with no tier lines", key)
			}
			d.format = tiers(names...)
		default:
			return nil, fmt.Errorf("properties.rec: %s: unknown kind %q", key, kind)
		}
		out[key] = d
	}
	return out, nil
}

// Text formats the property as its tooltip line.
func (p Property) Text() string { return propertyDefs[p.Key].format(p.Min, p.Max) }

// PropertyList lists an item's magical powers (a compact PrintItemPower), in tooltip order.
func (it *Item) PropertyList() []Property {
	var p []Property
	add := func(cond bool, key string, v int) {
		if cond {
			p = append(p, Property{key, v, v})
		}
	}
	addRange := func(cond bool, key string, lo, hi int) {
		if cond {
			p = append(p, Property{key, lo, hi})
		}
	}
	flag := func(bit uint32) bool { return it.Flags&bit != 0 }
	tier := func(bits ...uint32) int { // the highest set bit's tier, 0 = none
		t := 0
		for i, bit := range bits {
			if flag(bit) {
				t = i + 1
			}
		}
		return t
	}
	add(it.PLToHit != 0, "tohit", it.PLToHit)
	add(it.PLDam != 0, "dam%", it.PLDam)
	add(it.PLAC != 0, "ac%", it.PLAC)
	add(it.PLFR != 0, "fr", it.PLFR)
	add(it.PLLR != 0, "lr", it.PLLR)
	add(it.PLMR != 0, "mr", it.PLMR)
	add(it.PLStr != 0, "str", it.PLStr)
	add(it.PLMag != 0, "mag", it.PLMag)
	add(it.PLDex != 0, "dex", it.PLDex)
	add(it.PLVit != 0, "vit", it.PLVit)
	add(it.PLHP != 0, "hp", it.PLHP)
	add(it.PLMana != 0, "mana", it.PLMana)
	add(it.PLDamMod != 0, "dammod", it.PLDamMod)
	add(it.PLGetHit != 0, "gethit", it.PLGetHit)
	add(it.PLLight != 0, "light", it.PLLight)
	add(it.SplLvlAdd != 0, "spelllevels", it.SplLvlAdd)
	add(it.PLEnAc != 0, "enac", 1)
	addRange(flag(ISplFireDam), "firedam", it.FMinDam, it.FMaxDam)
	addRange(flag(ISplLightDam), "lightdam", it.LMinDam, it.LMaxDam)
	addRange(flag(ISplFireArrows), "firearrows", it.FMinDam, it.FMaxDam)
	addRange(flag(ISplLightArrows), "lightarrows", it.LMinDam, it.LMaxDam)
	add(flag(ISplStealLife3|ISplStealLife5), "stealslife", []int{0, 3, 5}[tier(ISplStealLife3, ISplStealLife5)])
	add(flag(ISplStealMana3|ISplStealMana5), "stealsmana", []int{0, 3, 5}[tier(ISplStealMana3, ISplStealMana5)])
	add(flag(ISplRndStealLife), "rndstealslife", 1)
	add(flag(ISplDrainLife), "drainlife", 1)
	add(flag(ISplQuickAttack|ISplFastAttack|ISplFasterAttack|ISplFastestAttack), "attackspeed",
		tier(ISplQuickAttack, ISplFastAttack, ISplFasterAttack, ISplFastestAttack))
	add(flag(ISplFastRecover|ISplFasterRecover|ISplFastestRecover), "recovery",
		tier(ISplFastRecover, ISplFasterRecover, ISplFastestRecover))
	add(flag(ISplFastBlock), "fastblock", 1)
	add(flag(ISplKnockback), "knockback", 1)
	add(flag(ISplThorns), "thorns", 1)
	add(flag(ISplNoMana), "nomana", 1)
	add(flag(ISplNoHealPlr), "nohealplr", 1)
	add(flag(ISplAbsHalfTrap), "abshalftrap", 1)
	add(flag(ISpl3xDamVsDemons), "3xdemons", 1)
	add(flag(ISplAllResZero), "allreszero", 1)
	add(flag(ISplInfravision), "infravision", 1)
	add(flag(ISplNoHealMon), "nohealmon", 1)
	add(flag(ISplRndArrowVel), "rndarrowvel", 1)
	add(it.MaxDur == 255, "indestructible", 1)
	return p
}

// compared lists what Compare looks at: Damage, Armor and, if affixes, the magical powers.
func (it *Item) compared(affixes bool) map[string]Property {
	m := map[string]Property{}
	if it.MaxDam > 0 {
		m[KeyDamage] = Property{KeyDamage, it.MinDam, it.MaxDam}
	}
	if it.AC > 0 {
		m[KeyArmor] = Property{KeyArmor, it.AC, it.AC}
	}
	if affixes {
		for _, p := range it.PropertyList() {
			m[p.Key] = p
		}
	}
	return m
}

// Compare rates each of it's Damage, Armor and (if affixes) property values against other's: +1 better, -1 worse,
// 0 same. A value other lacks counts as zero; ranges compare by midpoint. other == nil compares against nothing.
func (it *Item) Compare(other *Item, affixes bool) map[string]int {
	mine, theirs := it.compared(affixes), map[string]Property{}
	if other != nil {
		theirs = other.compared(affixes)
	}
	result := map[string]int{}
	for key, p := range mine {
		q := theirs[key]
		diff := p.Min + p.Max - q.Min - q.Max // twice the midpoint difference
		result[key] = propertyDefs[key].better * sign(diff)
	}
	return result
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// PropertyLine is a tooltip line with how its value compares (see Compare).
type PropertyLine struct {
	Text string
	Cmp  int
}

// PropertyLines formats PropertyList with each line's cmp; equal resists that compare alike merge into Resist All.
func (it *Item) PropertyLines(cmp map[string]int) []PropertyLine {
	allRes := it.PLFR == it.PLLR && it.PLLR == it.PLMR && cmp["fr"] == cmp["lr"] && cmp["lr"] == cmp["mr"]
	var lines []PropertyLine
	for _, p := range it.PropertyList() {
		if allRes && (p.Key == "lr" || p.Key == "mr") {
			continue
		}
		line := PropertyLine{p.Text(), cmp[p.Key]}
		if allRes && p.Key == "fr" {
			line.Text = fmt.Sprintf("Resist All: %+d%%", p.Min)
		}
		lines = append(lines, line)
	}
	return lines
}

// Properties describes an item's magical powers as tooltip lines.
func (it *Item) Properties() []string {
	var s []string
	for _, l := range it.PropertyLines(nil) {
		s = append(s, l.Text)
	}
	return s
}
