package d1

import (
	"cmp"
	"math/rand"
	"slices"
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
	storeMaxVal  = 140000
	boyMaxVal    = 90000
	premiumSlots = 6
)

var premiumLvlAdd = [premiumSlots]int{-1, -1, 0, 0, 1, 2}

func smithItemOk(b ItemData) bool {
	return b.Type != ITypeMisc && b.Type != ITypeGold && b.Type != ITypeStaff && b.Type != ITypeRing && b.Type != ITypeAmulet
}

func premiumItemOk(b ItemData) bool {
	return b.Type != ITypeMisc && b.Type != ITypeGold && b.Type != ITypeStaff
}

func witchItemOk(b ItemData) bool {
	switch {
	case b.MiscID == "IMISC_MANA", b.MiscID == "IMISC_FULLMANA", b.MiscID == "IMISC_HEAL", b.MiscID == "IMISC_FULLHEAL",
		b.Spell == "SPL_TOWN", b.Spell == "SPL_RESURRECT", b.Spell == "SPL_HEALOTHER":
		return false
	}
	return b.Type == ITypeMisc || b.Type == ITypeStaff
}

func healerItemOk(b ItemData) bool {
	if b.Type != ITypeMisc {
		return false
	}
	switch b.MiscID {
	case "IMISC_ELIXSTR", "IMISC_ELIXMAG", "IMISC_ELIXDEX", "IMISC_ELIXVIT", "IMISC_REJUV", "IMISC_FULLREJUV":
		return true
	case "IMISC_SCROLL":
		return b.Spell == "SPL_HEAL"
	}
	return false
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

// SmithStock ports SpawnSmith: 10-19 plain weapons and armor.
func (g *Generator) SmithStock(lvl int) []*Item {
	var out []*Item
	for n := g.random(10) + 10; n > 0; n-- {
		it := g.storeItem(g.storeIdx(lvl, smithItemOk, true), lvl)
		for it.IValue > storeMaxVal {
			it = g.storeItem(g.storeIdx(lvl, smithItemOk, true), lvl)
		}
		out = append(out, it)
	}
	sortByBase(out)
	return out
}

// PremiumItem ports SpawnOnePremium.
func (g *Generator) PremiumItem(plvl int) *Item {
	plvl = min(max(plvl, 1), 30)
	for {
		var l []int
		for i := 1; i < len(g.D.Items); i++ {
			b := g.D.Items[i]
			if b.Drop != "IDROP_NEVER" && premiumItemOk(b) && b.MinMLvl >= plvl>>2 && b.MinMLvl <= plvl {
				l = append(l, i)
			}
		}
		it := g.storeItem(l[g.random(len(l))], plvl)
		g.itemBonus(it, plvl>>1, plvl, true)
		if it.IValue <= storeMaxVal {
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

// RefillPremium ports SpawnPremium: fill bought slots, then rotate in new items while the character outlevels the stock.
func (g *Generator) RefillPremium(p *Premium, clvl int) {
	p.Level = max(p.Level, 1)
	for i, it := range p.Items {
		if it == nil {
			p.Items[i] = g.PremiumItem(p.Level + premiumLvlAdd[i])
		}
	}
	for p.Level < clvl {
		p.Level++
		p.Items[0], p.Items[1], p.Items[2] = p.Items[2], p.Items[3], p.Items[4]
		p.Items[3] = g.PremiumItem(p.Level + premiumLvlAdd[3])
		p.Items[4] = p.Items[5]
		p.Items[5] = g.PremiumItem(p.Level + premiumLvlAdd[5])
	}
}

// WitchStock ports SpawnWitch: mana potions and town portal, then 7-14 random misc items and staves.
// spellLevels are the hero's, for WitchBookLevel.
func (g *Generator) WitchStock(lvl int, spellLevels [37]int) []*Item {
	out := []*Item{g.storeItem(IdxMana, 1), g.storeItem(IdxFullMana, 1), g.storeItem(IdxPortal, 1)}
	var rnd []*Item
	for n := g.random(8) + 10; n > 3; n-- {
		var it *Item
		for it == nil || it.IValue > storeMaxVal {
			it = g.storeItem(g.storeIdx(lvl, witchItemOk, false), lvl)
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
func (g *Generator) witchBookLevel(it *Item, spellLevels [37]int) {
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

// HealerStock ports SpawnHealer: healing potions, then 8-15 random elixirs, rejuvenation potions and healing scrolls.
func (g *Generator) HealerStock(lvl int) []*Item {
	out := []*Item{g.storeItem(IdxHeal, 1), g.storeItem(IdxFullHeal, 1)}
	var rnd []*Item
	for n := g.random(8) + 10; n > 2; n-- {
		rnd = append(rnd, g.storeItem(g.storeIdx(lvl, healerItemOk, false), lvl))
	}
	sortByBase(rnd)
	return append(out, rnd...)
}

// BoyItem ports SpawnBoy's roll (Wirt's single magic item).
func (g *Generator) BoyItem(clvl int) *Item {
	for {
		it := g.storeItem(g.storeIdx(clvl, premiumItemOk, false), clvl)
		g.itemBonus(it, clvl, 2*clvl, true)
		if it.IValue <= boyMaxVal {
			it.Identified = true
			return it
		}
	}
}

// BuyPrice is what a store charges (the identified value).
func BuyPrice(it *Item) int { return it.IValue }

// WirtPrice ports S_BBuyEnter: Wirt adds 50%.
func WirtPrice(it *Item) int { return it.IValue + it.IValue>>1 }

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

// SmithBuys ports SmithSellOk.
func SmithBuys(it *Item) bool {
	return it.Type != ITypeMisc && it.Type != ITypeGold && it.Type != ITypeStaff && it.BaseIdx != IdxLazStaff
}

// WitchBuys ports WitchSellOk.
func WitchBuys(it *Item) bool {
	return (it.Type == ITypeMisc || it.Type == ITypeStaff) && !isQuestItem(it)
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

// CanRecharge ports WitchRechargeOk.
func CanRecharge(it *Item) bool {
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
