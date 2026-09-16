package d1

import "fmt"

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

func signed(format string) func(int, int) string {
	return func(v, _ int) string { return fmt.Sprintf(format, v) }
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

var propertyDefs = map[string]propertyDef{
	KeyDamage:        {ranged("Damage: %d-%d"), 1},
	KeyArmor:         {signed("Armor: %d"), 1},
	"tohit":          {signed("chance to hit: %+d%%"), 1},
	"dam%":           {signed("%+d%% damage"), 1},
	"ac%":            {signed("%+d%% armor"), 1},
	"fr":             {signed("Resist Fire: %+d%%"), 1},
	"lr":             {signed("Resist Lightning: %+d%%"), 1},
	"mr":             {signed("Resist Magic: %+d%%"), 1},
	"str":            {signed("%+d to strength"), 1},
	"mag":            {signed("%+d to magic"), 1},
	"dex":            {signed("%+d to dexterity"), 1},
	"vit":            {signed("%+d to vitality"), 1},
	"hp":             {signed("Hit Points: %+d"), 1},
	"mana":           {signed("Mana: %+d"), 1},
	"dammod":         {signed("adds %d points to damage"), 1},
	"gethit":         {signed("%+d damage from enemies"), -1},
	"light":          {func(v, _ int) string { return fmt.Sprintf("%+d%% light radius", v*10) }, 1},
	"spelllevels":    {signed("%+d to spell levels"), 1},
	"enac":           {text("decreases target's armor"), 1},
	"firedam":        {ranged("fire hit damage: %d-%d"), 1},
	"lightdam":       {ranged("lightning hit damage: %d-%d"), 1},
	"firearrows":     {ranged("fire arrows damage: %d-%d"), 1},
	"lightarrows":    {ranged("lightning arrows damage: %d-%d"), 1},
	"stealslife":     {signed("steals %d%% life"), 1},
	"stealsmana":     {signed("steals %d%% mana"), 1},
	"rndstealslife":  {text("hit steals random life"), 1},
	"drainlife":      {text("drains life"), 1},
	"attackspeed":    {tiers("quick attack", "fast attack", "faster attack", "fastest attack"), 1},
	"recovery":       {tiers("fast hit recovery", "faster hit recovery", "fastest hit recovery"), 1},
	"fastblock":      {text("fast block"), 1},
	"knockback":      {text("knocks target back"), 1},
	"thorns":         {text("attacker takes 1-3 damage"), 1},
	"nomana":         {text("user loses all mana"), -1},
	"nohealplr":      {text("you can't heal"), -1},
	"abshalftrap":    {text("absorbs half of trap damage"), 1},
	"3xdemons":       {text("+200% damage vs. demons"), 1},
	"allreszero":     {text("all resistance equals 0"), -1},
	"infravision":    {text("see with infravision"), 1},
	"nohealmon":      {text("hit monster doesn't heal"), 1},
	"rndarrowvel":    {text("fires random speed arrows"), 0},
	"indestructible": {text("indestructible"), 1},
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
