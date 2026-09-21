package d1

import (
	"cmp"
	"fmt"
	"io/fs"
	"math/rand"
	"slices"
	"strings"
)

// Item indexes (IDI_) used by stores and starting gear.
const (
	IdxGold      = 0
	IdxHeal      = 24
	IdxMana      = 25
	IdxPortal    = 27
	IdxFullHeal  = 29
	IdxFullMana  = 30
	IdxLazStaff  = 33
	idxFirstQst  = 6
	idxLastQst   = 22
	premiumSlots = 15 // Hellfire's SMITH_PREMIUM_ITEMS; D1 uses the first 6
)

// storeLimits are defs.h's store sizes and value caps (Hellfire's).
type storeLimits struct {
	smithItems, witchItems, premiumItems int
	maxVal, boyMaxVal                    int
	premiumLvlAdd                        []int
}

var hfStoreLimits = storeLimits{25, 25, 15, 200000, 200000, []int{-1, -1, -1, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 3, 3}}

func (g *Generator) limits() storeLimits { return hfStoreLimits }

// Shopper is what Hellfire's stores look at of the hero: its class and stats for what it could use, and what it
// already owns, so nothing offered is worse than its best item of a kind.
type Shopper struct {
	Class                              int
	Str, Mag, Dex                      int // with items
	BaseStr, BaseMag, BaseDex, BaseVit int
	Owned                              []*Item // equipped and in the pack
}

func (g *Generator) smithItemOk(b ItemData) bool {
	staff := b.Type == ITypeStaff && b.Spell != "SPL_NULL" // Hellfire's smith sells plain staves
	return b.Type != ITypeMisc && b.Type != ITypeGold && !staff && b.Type != ITypeRing && b.Type != ITypeAmulet
}

func (g *Generator) premiumItemOk(b ItemData) bool {
	return b.Type != ITypeMisc && b.Type != ITypeGold
}

func (g *Generator) witchItemOk(b ItemData) bool {
	switch {
	case b.MiscID == "IMISC_MANA", b.MiscID == "IMISC_FULLMANA", b.MiscID == "IMISC_HEAL", b.MiscID == "IMISC_FULLHEAL",
		b.Spell == "SPL_TOWN", b.Spell == "SPL_RESURRECT", b.Spell == "SPL_HEALOTHER", IsOil(b.MiscID):
		return false
	}
	return b.Type == ITypeMisc || b.Type == ITypeStaff
}

// healerItemOk ports HealerItemOk; Hellfire only sells an elixir while the stat is below the class maximum.
func (g *Generator) healerItemOk(b ItemData, who *Shopper) bool {
	if b.Type != ITypeMisc {
		return false
	}
	switch b.MiscID {
	case "IMISC_ELIXSTR", "IMISC_ELIXMAG", "IMISC_ELIXDEX", "IMISC_ELIXVIT":
		if who == nil {
			return true
		}
		a := classAttr[who.Class]
		return map[string]bool{"IMISC_ELIXSTR": who.BaseStr < a.MaxStr, "IMISC_ELIXMAG": who.BaseMag < a.MaxMag,
			"IMISC_ELIXDEX": who.BaseDex < a.MaxDex, "IMISC_ELIXVIT": who.BaseVit < a.MaxVit}[b.MiscID]
	case "IMISC_REJUV", "IMISC_FULLREJUV":
		return true
	case "IMISC_SCROLL":
		return b.Spell == "SPL_HEAL"
	}
	return false
}

// bestOwned ports get_*_max_value: the highest identified value among the hero's items of the same kind as it.
func (who *Shopper) bestOwned(it *Item) int {
	kind := func(t int) int {
		if t == ITypeLArmor || t == ITypeMArmor || t == ITypeHArmor {
			return ITypeLArmor
		}
		return t
	}
	switch it.Type {
	case ITypeLArmor, ITypeMArmor, ITypeHArmor, ITypeShield, ITypeAxe, ITypeBow, ITypeMace, ITypeSword, ITypeHelm,
		ITypeStaff, ITypeRing, ITypeAmulet:
	default:
		return 0
	}
	best := 0
	for _, o := range who.Owned {
		if kind(o.Type) == kind(it.Type) {
			best = max(best, o.IValue)
		}
	}
	return best
}

// fitsShopper is Hellfire's SpawnOnePremium/SpawnBoy acceptance: within 1.2× the higher of the hero's stats and its
// class maximum, and worth at least 80% of its best item of the kind.
func (who *Shopper) fits(it *Item, maxVal, floorValue int) bool {
	a := classAttr[who.Class]
	str, mag, dex := max(a.MaxStr, who.Str)*12/10, max(a.MaxMag, who.Mag)*12/10, max(a.MaxDex, who.Dex)*12/10
	return it.IValue <= maxVal && it.MinStr <= str && it.MinMag <= mag && it.MinDex <= dex && it.IValue >= floorValue
}

// boyShuns are the item types Hellfire's Wirt keeps away from each class (for his first 200 tries). Filled by
// Load from the boy_shuns section of assets/data/stores.rec.
var boyShuns [NumClasses][]int

// itemTypeByName names the ITYPE_ values the store tables refer to.
var itemTypeByName = map[string]int{"misc": ITypeMisc, "sword": ITypeSword, "axe": ITypeAxe, "bow": ITypeBow,
	"mace": ITypeMace, "shield": ITypeShield, "larmor": ITypeLArmor, "helm": ITypeHelm, "marmor": ITypeMArmor,
	"harmor": ITypeHArmor, "staff": ITypeStaff, "gold": ITypeGold, "ring": ITypeRing, "amulet": ITypeAmulet,
	"food": ITypeFood}

func loadStoreTables(fsys fs.FS) error {
	recs, err := readRecMulti(fsys, "stores.rec")
	if err != nil {
		return err
	}
	shuns := recs["boy_shuns"]
	if len(shuns) != NumClasses {
		return fmt.Errorf("stores.rec: %d boy_shuns rows, want %d", len(shuns), NumClasses)
	}
	for i, r := range shuns {
		m := r.ToMap("")
		boyShuns[i] = nil
		for _, name := range strings.Fields(m.GetStringOrDefault("shuns", "")) {
			t, ok := itemTypeByName[name]
			if !ok {
				return fmt.Errorf("stores.rec: unknown item type %q", name)
			}
			boyShuns[i] = append(boyShuns[i], t)
		}
	}
	return nil
}

// storeIdx ports RndSmithItem/RndWitchItem/RndHealerItem/RndBoyItem.
func (g *Generator) storeIdx(lvl int, ok func(ItemData) bool, double bool) int {
	var l []int
	for i := 1; i < len(g.D.Items); i++ {
		b := g.D.Items[i]
		if b.Drop != "IDROP_NEVER" && ok(b) && lvl >= b.MinMLvl {
			l = append(l, i)
			if double && b.Drop == "IDROP_DOUBLE" {
				l = append(l, i)
			}
		}
	}
	return l[g.random(len(l))]
}

func (g *Generator) storeItem(idx, lvl int) *Item {
	it := g.itemAttrs(idx, lvl, 1, false)
	it.Identified = true
	return it
}

func sortByBase(items []*Item) {
	slices.SortStableFunc(items, func(a, b *Item) int { return cmp.Compare(a.BaseIdx, b.BaseIdx) })
}

// StoreLevel ports SetupTownStores' item level: deepest visited level + 2, between 6 and 16.
func StoreLevel(deepestLevel int) int { return min(max(deepestLevel+2, 6), 16) }

// SmithStock ports SpawnSmith: 10-19 plain weapons and armor (Hellfire: 10-24, and plain staves).
func (g *Generator) SmithStock(lvl int) []*Item {
	var out []*Item
	lim := g.limits()
	for n := g.random(lim.smithItems-10) + 10; n > 0; n-- {
		it := g.storeItem(g.storeIdx(lvl, g.smithItemOk, true), lvl)
		for it.IValue > lim.maxVal {
			it = g.storeItem(g.storeIdx(lvl, g.smithItemOk, true), lvl)
		}
		out = append(out, it)
	}
	sortByBase(out)
	return out
}

// PremiumItem ports SpawnOnePremium. Hellfire's staves take a power rather than a spell, and it tries up to 150
// times for an item the shopper could use and that beats 80% of its best one of the kind.
func (g *Generator) PremiumItem(plvl int, who *Shopper) *Item {
	plvl = min(max(plvl, 1), 30)
	lim := g.limits()
	for count := 1; ; count++ {
		var l []int
		for i := 1; i < len(g.D.Items); i++ {
			b := g.D.Items[i]
			if b.Drop != "IDROP_NEVER" && g.premiumItemOk(b) && b.MinMLvl >= plvl>>2 && b.MinMLvl <= plvl {
				l = append(l, i)
			}
		}
		it := g.storeItem(l[g.random(len(l))], plvl)
		g.itemBonusSpells(it, plvl>>1, plvl, true, false)
		ok := it.IValue <= lim.maxVal
		if who != nil {
			ok = who.fits(it, lim.maxVal, who.bestOwned(it)*8/10) || count >= 150
		}
		if ok {
			it.Identified = true
			return it
		}
	}
}

// Premium is Griswold's rotating premium stock; it is game state and must be saved.
type Premium struct {
	Items [premiumSlots]*Item
	Level int
}

// RefillPremium ports SpawnPremium: fill bought slots, then rotate in new items while the character outlevels the
// stock (D1 moves two along a level, Hellfire three).
func (g *Generator) RefillPremium(p *Premium, clvl int, who *Shopper) {
	lim := g.limits()
	p.Level = max(p.Level, 1)
	add := lim.premiumLvlAdd
	for i := range p.Items[:lim.premiumItems] {
		if p.Items[i] == nil {
			p.Items[i] = g.PremiumItem(p.Level+add[i], who)
		}
	}
	for p.Level < clvl {
		p.Level++
		it := &p.Items
		copy(it[0:10], it[3:13])
		it[10] = g.PremiumItem(p.Level+add[10], who)
		it[11] = it[13]
		it[12] = g.PremiumItem(p.Level+add[12], who)
		it[13] = it[14]
		it[14] = g.PremiumItem(p.Level+add[14], who)
	}
}

// WitchStock ports SpawnWitch: mana potions and town portal, then 7-14 random misc items and staves. Hellfire's
// Adria stocks up to 24 items, the first up to three of them books from its four plain book rows.
// spellLevels are the hero's, for WitchBookLevel.
func (g *Generator) WitchStock(lvl int, spellLevels [NumSpells]int) []*Item {
	lim := g.limits()
	count, books := g.random(lim.witchItems-10)+10, g.random(4)
	out := []*Item{g.storeItem(IdxMana, 1), g.storeItem(IdxFullMana, 1), g.storeItem(IdxPortal, 1)}
	var rnd []*Item
	for i := 114; i <= 117 && len(rnd) < books; i++ { // Hellfire's "Book of" rows
		if b := g.D.Items[i]; g.witchItemOk(b) && lvl >= b.MinMLvl {
			g.random(1) // SpawnWitch's volatile junk roll
			it := g.storeItem(i, lvl)
			it.Identified = true
			g.witchBookLevel(it, spellLevels)
			rnd = append(rnd, it)
		}
	}
	for n := count - len(rnd); n > 3; n-- {
		var it *Item
		for it == nil || it.IValue > lim.maxVal {
			it = g.storeItem(g.storeIdx(lvl, g.witchItemOk, false), lvl)
			iblvl := -1
			if g.random(100) <= 5 || it.MiscID == "IMISC_STAFF" {
				iblvl = 2 * lvl
			}
			if iblvl != -1 {
				g.itemBonus(it, iblvl>>1, iblvl, true)
			}
		}
		it.Identified = true
		g.witchBookLevel(it, spellLevels)
		rnd = append(rnd, it)
	}
	sortByBase(rnd)
	return append(out, rnd...)
}

// witchBookLevel ports WitchBookLevel: a book in Adria's stock asks 20% more Magic per level the hero already has
// in its spell, up to 255.
func (g *Generator) witchBookLevel(it *Item, spellLevels [NumSpells]int) {
	if it.MiscID != "IMISC_BOOK" {
		return
	}
	it.MinMag = g.D.Spells[it.Spell].MinInt
	for slvl := spellLevels[it.Spell]; slvl != 0; {
		it.MinMag += 20 * it.MinMag / 100
		slvl--
		if it.MinMag+20*it.MinMag/100 > 255 {
			it.MinMag, slvl = 255, 0
		}
	}
}

// HealerStock ports SpawnHealer: healing potions, then 8-15 random elixirs, rejuvenation potions and healing scrolls
// (Hellfire: 8-17, and only the elixirs the shopper can still use).
func (g *Generator) HealerStock(lvl int, who *Shopper) []*Item {
	out := []*Item{g.storeItem(IdxHeal, 1), g.storeItem(IdxFullHeal, 1)}
	var rnd []*Item
	n := g.random(10) + 10
	ok := func(b ItemData) bool { return g.healerItemOk(b, who) }
	for ; n > 2; n-- {
		rnd = append(rnd, g.storeItem(g.storeIdx(lvl, ok, false), lvl))
	}
	sortByBase(rnd)
	return append(out, rnd...)
}

// BoyItem ports SpawnBoy's roll (Wirt's single magic item). Hellfire's Wirt tries up to 250 times for an item the
// shopper could use, beating 80% of its best one of the kind and, for his first 200 tries, of a type its class uses.
func (g *Generator) BoyItem(clvl int, who *Shopper) *Item {
	lim := g.limits()
	for count := 1; ; count++ {
		it := g.storeItem(g.storeIdx(clvl, g.premiumItemOk, false), clvl)
		g.itemBonus(it, clvl, 2*clvl, true)
		ok := it.IValue <= lim.boyMaxVal
		if who != nil {
			floor := who.bestOwned(it) * 8 / 10
			if count < 200 && slices.Contains(boyShuns[who.Class], it.Type) {
				floor = 1 << 62
			}
			ok = who.fits(it, lim.boyMaxVal, floor) || count >= 250
		}
		if ok {
			it.Identified = true
			return it
		}
	}
}

// BuyPrice is what a store charges (the identified value).
func BuyPrice(it *Item) int { return it.IValue }

// WirtPrice ports S_BBuyEnter: Hellfire's Wirt takes 25% off.
func (d *Data) WirtPrice(it *Item) int { return it.IValue - it.IValue>>2 }

// SellPrice ports S_StartSSell/S_StartWSell: a quarter of the (identified) value, at least 1.
func SellPrice(it *Item) int {
	v := it.Value
	if it.Magical != 0 && it.Identified {
		v = it.IValue
	}
	return max(v>>2, 1)
}

func isQuestItem(it *Item) bool {
	return it.BaseIdx >= idxFirstQst && it.BaseIdx <= idxLastQst || it.BaseIdx == IdxLazStaff
}

// SmithBuys ports SmithSellOk; Hellfire's Griswold also buys oils and plain staves, but no quest items.
func (d *Data) SmithBuys(it *Item) bool {
	switch {
	case IsOil(it.MiscID):
		return true
	case it.Type == ITypeMisc, it.Type == ITypeGold, it.Type == ITypeStaff && it.Spell != 0, it.Class == "ICLASS_QUEST":
		return false
	}
	return it.BaseIdx != IdxLazStaff
}

// WitchBuys ports WitchSellOk; Hellfire's Adria takes no oils or quest items, and only staves with a spell.
func (d *Data) WitchBuys(it *Item) bool {
	ok := it.Type == ITypeMisc && !IsOil(it.MiscID) && it.Class != "ICLASS_QUEST" || it.Type == ITypeStaff && it.Spell != 0
	return ok && !isQuestItem(it)
}

// CanRepair ports SmithRepairOk.
func CanRepair(it *Item) bool {
	return it.MaxDur > 0 && it.Durability != it.MaxDur && it.Type != ITypeMisc && it.Type != ITypeGold
}

// RepairItem ports the Warrior's Item Repair (RepairItem): each round of repair costs maximum durability, and an
// item worn down to none is gone (it reports true). rnd is random_.
func RepairItem(it *Item, clvl int, rnd func(int) int) (destroyed bool) {
	if it.Durability == it.MaxDur {
		return false
	}
	if it.MaxDur <= 0 {
		return true
	}
	rep := 0
	for {
		rep += clvl + rnd(clvl)
		it.MaxDur -= max(it.MaxDur/(clvl+9), 1)
		if it.MaxDur == 0 {
			return true
		}
		if rep+it.Durability >= it.MaxDur {
			break
		}
	}
	it.Durability = min(it.Durability+rep, it.MaxDur)
	return false
}

// RechargeItem ports the Sorcerer's Staff Recharge (DoRecharge/RechargeItem): each round adds up to clvl/sBookLvl
// charges and costs one maximum charge. rnd is random_, which gives 0 for a range of 0 or less.
func (d *Data) RechargeItem(it *Item, clvl int, rnd func(int) int) {
	if it.Type != ITypeStaff || it.Spell == 0 || it.Charges == it.MaxCharges {
		return
	}
	r := 1
	if bl := d.Spells[it.Spell].BookLvl; bl > 0 && clvl/bl > 0 {
		r += rnd(clvl / bl)
	}
	for {
		it.MaxCharges--
		if it.MaxCharges == 0 {
			return
		}
		it.Charges += r
		if it.Charges >= it.MaxCharges {
			break
		}
	}
	it.Charges = min(it.Charges, it.MaxCharges)
}

// DisarmChance ports TryDisarm's roll: disarming works if random_(154, 100) <= the result.
func DisarmChance(dex, dlvl int) int { return 2*dex - 5*dlvl }

// RepairCost ports AddStoreHoldRepair.
func RepairCost(it *Item) int {
	value := it.Value
	if it.Magical != 0 && it.Identified {
		value = 30 * it.IValue / 100
	}
	v := 100 * (it.MaxDur - it.Durability) / it.MaxDur * value / 100
	if v > 1 {
		v >>= 1
	}
	return max(v, 1)
}

// CanRecharge ports WitchRechargeOk; Hellfire's Adria also recharges uniques with charges.
func (d *Data) CanRecharge(it *Item) bool {
	if (it.MiscID == "IMISC_UNIQUE" || it.MiscID == "IMISC_STAFF") && it.Charges < it.MaxCharges {
		return true
	}
	return it.Type == ITypeStaff && it.MaxCharges > 0 && it.Charges != it.MaxCharges
}

// RechargeCost ports AddStoreHoldRecharge.
func (d *Data) RechargeCost(it *Item) int {
	v := it.Value + d.Spells[it.Spell].StaffCost
	return 100 * (it.MaxCharges - it.Charges) / it.MaxCharges * v / 100 >> 1
}

// wear takes one point of durability; true when the item just broke.
func wear(it *Item) bool {
	if it == nil || it.MaxDur == 0 || it.Durability == 255 || it.Durability == 0 {
		return false
	}
	it.Durability--
	return it.Durability == 0
}

// WeaponDur ports WeaponDur: with a 1 in durrnd chance the weapon (or a lone shield) wears. Returns the item that broke.
func WeaponDur(rng *rand.Rand, equipped []*Item, durrnd int) *Item {
	if rng.Intn(durrnd) != 0 {
		return nil
	}
	var target *Item
	for _, it := range equipped {
		if it.Class == "ICLASS_WEAPON" || target == nil && it.Type == ITypeShield {
			target = it
		}
	}
	if wear(target) {
		return target
	}
	return nil
}

// ArmorDur ports ArmorDur: body armor or helm wears when the hero is hit. Returns the item that broke.
func ArmorDur(rng *rand.Rand, equipped []*Item) *Item {
	var body, helm *Item
	for _, it := range equipped {
		switch it.Type {
		case ITypeLArmor, ITypeMArmor, ITypeHArmor:
			body = it
		case ITypeHelm:
			helm = it
		}
	}
	if body == nil && helm == nil {
		return nil
	}
	target := helm
	if rng.Intn(3) != 0 && helm != nil && body != nil || helm == nil {
		target = body
	}
	if wear(target) {
		return target
	}
	return nil
}

// ShieldDur ports ShieldDur: the shield wears on every block. Returns the item that broke.
func ShieldDur(equipped []*Item) *Item {
	for _, it := range equipped {
		if it.Type == ITypeShield && wear(it) {
			return it
		}
	}
	return nil
}
