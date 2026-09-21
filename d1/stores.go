package d1

import (
	"cmp"
	"fmt"
	"io/fs"
	"math/rand"
	"slices"

	"rpg/defs"
	"rpg/stats"
)

// Item indexes (IDI_) used by generation and starting gear.
const (
	IdxGold      = 0
	IdxHeal      = 24
	IdxMana      = 25
	IdxPortal    = 27
	IdxLazStaff  = 33
	idxFirstQst  = 6
	idxLastQst   = 22
	premiumSlots = 15 // the most fixed slots a rotating stock may declare (SMITH_PREMIUM_ITEMS: 6 in 1.09, 15 in Hellfire)
)

// Shopper is what the stores look at of the hero: its class and stats for what it could use, and what it
// already owns, so nothing offered is worse than its best item of a kind.
type Shopper struct {
	Class                              int
	Str, Mag, Dex                      int // with items
	BaseStr, BaseMag, BaseDex, BaseVit int
	Owned                              []*Item // equipped and in the pack
}

// usableElixir is Hellfire's HealerItemOk rule a vendor opts into with usable_elixirs_only: an elixir is only worth
// selling while the hero is still under its class maximum.
func usableElixir(b ItemData, who *Shopper) bool {
	if who == nil {
		return true
	}
	a := classAttr[who.Class]
	switch b.MiscID {
	case "IMISC_ELIXSTR":
		return who.BaseStr < a.MaxStr
	case "IMISC_ELIXMAG":
		return who.BaseMag < a.MaxMag
	case "IMISC_ELIXDEX":
		return who.BaseDex < a.MaxDex
	case "IMISC_ELIXVIT":
		return who.BaseVit < a.MaxVit
	}
	return true
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

// fits is Hellfire's SpawnOnePremium/SpawnBoy acceptance, for a vendor with tries: within 1.2x the higher of the
// hero's stats and its class maximum, and worth at least 80% of its best item of the kind.
func (who *Shopper) fits(it *Item, maxVal, floorValue int) bool {
	a := classAttr[who.Class]
	str, mag, dex := max(a.MaxStr, who.Str)*12/10, max(a.MaxMag, who.Mag)*12/10, max(a.MaxDex, who.Dex)*12/10
	return it.IValue <= maxVal && it.MinStr <= str && it.MinMag <= mag && it.MinDex <= dex && it.IValue >= floorValue
}

// loadStoreTables reads the mod's vendors. One loader, one record, same as monsters and items -- and after the
// mod has been chosen, never at init.
func loadStoreTables(fsys fs.FS, d *Data) error {
	vendors, err := defs.ReadVendors(fsys)
	if err != nil {
		return err
	}
	d.Vendors = make(map[string]Vendor, len(vendors))
	for _, v := range vendors {
		if len(v.SlotLevels) > premiumSlots {
			return fmt.Errorf("stores.rec: %s declares %d slots, at most %d fit", v.InternalName, len(v.SlotLevels), premiumSlots)
		}
		if v.Stock == "premium" && (v.RotateBy < 1 || len(v.SlotLevels) < 2*v.RotateBy-1) {
			return fmt.Errorf("stores.rec: %s rotates by %d, which %d slots can't hold", v.InternalName, v.RotateBy, len(v.SlotLevels))
		}
		d.Vendors[v.InternalName] = v
	}
	d.VendorList = vendors
	return nil
}

// idx resolves an item a vendor record names. A name the mod's items.rec does not have is a broken mod, and the
// store would silently sell gold instead, so say so.
func (d *Data) idx(internalName string) int {
	i, ok := d.ItemIdx[internalName]
	if !ok {
		panic(fmt.Sprintf("stores.rec names item %q, which this mod's items.rec does not have", internalName))
	}
	return i
}

// rollCount rolls a stock size out of the record's interval, both ends included.
func (g *Generator) rollCount(i stats.Interval) int {
	return g.random(i.Max()-i.Min()+1) + i.Min()
}

// storeIdx ports RndSmithItem/RndWitchItem/RndHealerItem/RndBoyItem, with the four Go predicates replaced by the
// vendor's own criteria. The table is walked in index order, so the candidate list -- and the roll off it -- is
// the same every run.
func (g *Generator) storeIdx(v Vendor, lvl int, extra func(ItemData) bool) int {
	var l []int
	for i := 1; i < len(g.D.Items); i++ {
		b := g.D.Items[i]
		switch {
		case b.Drop == "IDROP_NEVER", lvl < b.MinMLvl, !v.Accepts(b):
			continue
		case v.MinDLvlDivisor > 0 && b.MinMLvl < lvl/v.MinDLvlDivisor:
			continue
		case extra != nil && !extra(b):
			continue
		}
		l = append(l, i)
		if v.DoubleDrops && b.Drop == "IDROP_DOUBLE" {
			l = append(l, i)
		}
	}
	return l[g.random(len(l))]
}

// always is the vendor's fixed stock, in the order the record lists it.
func (g *Generator) always(v Vendor) []*Item {
	out := make([]*Item, 0, len(v.Always))
	for _, name := range v.Always {
		out = append(out, g.storeItem(g.D.idx(name), 1))
	}
	return out
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

// SmithStock ports SpawnSmith: as many plain weapons and armor as the smith's record asks for.
func (g *Generator) SmithStock(lvl int) []*Item {
	v := g.D.Vendors["smith"]
	var out []*Item
	for n := g.rollCount(v.ItemCount); n > 0; n-- {
		it := g.storeItem(g.storeIdx(v, lvl, nil), lvl)
		for it.IValue > v.MaxValue {
			it = g.storeItem(g.storeIdx(v, lvl, nil), lvl)
		}
		out = append(out, it)
	}
	sortByBase(out)
	return out
}

// PremiumItem ports SpawnOnePremium: one magical item for a fixed slot, rolled until it is worth no more than the
// record's max_value. With tries it must also be something the shopper could use that beats 80% of its best one of
// the kind, settling after that many rolls (Hellfire).
func (g *Generator) PremiumItem(plvl int, who *Shopper) *Item {
	v := g.D.Vendors["premium"]
	plvl = min(max(plvl, 1), 30)
	for count := 1; ; count++ {
		it := g.storeItem(g.storeIdx(v, plvl, nil), plvl)
		g.itemBonus(it, plvl>>1, plvl, true)
		ok := it.IValue <= v.MaxValue
		if v.Tries > 0 && who != nil {
			ok = who.fits(it, v.MaxValue, who.bestOwned(it)*8/10) || count >= v.Tries
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
// stock. Each slot's item level offset is the record's slot_levels. Every level shifts the stock rotate_by slots
// down; the last 2*rotate_by-1 slots then alternate a new item and one kept from further up (1.09: 2 of 6 slots,
// Hellfire: 3 of 15).
func (g *Generator) RefillPremium(p *Premium, clvl int, who *Shopper) {
	v := g.D.Vendors["premium"]
	add, k := v.SlotLevels, v.RotateBy
	n := len(add)
	p.Level = max(p.Level, 1)
	for i := range p.Items[:n] {
		if p.Items[i] == nil {
			p.Items[i] = g.PremiumItem(p.Level+add[i], who)
		}
	}
	for p.Level < clvl {
		p.Level++
		old := p.Items
		head := n - (2*k - 1)
		copy(p.Items[:head], old[k:k+head])
		for j := 0; j < 2*k-1; j++ {
			if j%2 == 0 {
				p.Items[head+j] = g.PremiumItem(p.Level+add[head+j], who)
			} else {
				p.Items[head+j] = old[n-k+1+j/2]
			}
		}
	}
}

// WitchStock ports SpawnWitch: the record's always stock, then up to featured_count of its featured kind (Hellfire's
// books, whose price in Magic depends on the hero) and random misc items and staves for the rest. spellLevels are
// the hero's, for WitchBookLevel.
func (g *Generator) WitchStock(lvl int, spellLevels [NumSpells]int) []*Item {
	v := g.D.Vendors["witch"]
	count, featured := g.rollCount(v.ItemCount), 0
	if v.FeaturedMiscID != "" {
		featured = g.rollCount(v.FeaturedCount)
	}
	out := g.always(v)
	var rnd []*Item
	for i := 1; i < len(g.D.Items) && len(rnd) < featured; i++ {
		b := g.D.Items[i]
		if b.MiscID != v.FeaturedMiscID || !v.Accepts(b) || lvl < b.MinMLvl {
			continue
		}
		g.random(1) // SpawnWitch's volatile junk roll
		it := g.storeItem(i, lvl)
		it.Identified = true
		g.witchBookLevel(it, spellLevels)
		rnd = append(rnd, it)
	}
	for n := count - len(rnd); n > len(v.Always); n-- {
		var it *Item
		for it == nil || it.IValue > v.MaxValue {
			it = g.storeItem(g.storeIdx(v, lvl, nil), lvl)
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

// HealerStock ports SpawnHealer: the record's always stock, then random elixirs, rejuvenation potions and healing
// scrolls; with usable_elixirs_only, only the elixirs the shopper can still use.
func (g *Generator) HealerStock(lvl int, who *Shopper) []*Item {
	v := g.D.Vendors["healer"]
	out := g.always(v)
	var rnd []*Item
	var ok func(ItemData) bool
	if v.UsableElixirs {
		ok = func(b ItemData) bool { return usableElixir(b, who) }
	}
	for n := g.rollCount(v.ItemCount); n > len(v.Always); n-- {
		rnd = append(rnd, g.storeItem(g.storeIdx(v, lvl, ok), lvl))
	}
	sortByBase(rnd)
	return append(out, rnd...)
}

// BoyItem ports SpawnBoy's roll (Wirt's single magic item): rolled until it is worth no more than the record's
// max_value. With tries it must also be something the shopper could use that beats 80% of its best one of the kind
// and, for its first shun_tries, of a type its class uses (Hellfire).
func (g *Generator) BoyItem(clvl int, who *Shopper) *Item {
	v := g.D.Vendors["boy"]
	for count := 1; ; count++ {
		it := g.storeItem(g.storeIdx(v, clvl, nil), clvl)
		g.itemBonus(it, clvl, 2*clvl, true)
		ok := it.IValue <= v.MaxValue
		if v.Tries > 0 && who != nil {
			floor := who.bestOwned(it) * 8 / 10
			if count < v.ShunTries && slices.Contains(v.Shuns[ClassNames[who.Class]], defs.ItemTypeNames[it.Type]) {
				floor = 1 << 62
			}
			ok = who.fits(it, v.MaxValue, floor) || count >= v.Tries
		}
		if ok {
			it.Identified = true
			return it
		}
	}
}

// VendorsOf is every record the town NPC npc runs, in file order.
func (d *Data) VendorsOf(npc string) []Vendor {
	var out []Vendor
	for _, v := range d.VendorList {
		if slices.Contains(v.NPCs, npc) {
			out = append(out, v)
		}
	}
	return out
}

// ListStock is a vendor that names its wares: every always item, then sometimes items to fill the rolled count,
// at least one when it has any -- main's getVendorInventoryFromDefinition.
func (g *Generator) ListStock(v Vendor, lvl int) []*Item {
	count := max(len(v.Always), g.rollCount(v.ItemCount))
	out := g.always(v)
	if len(v.Sometimes) > 0 {
		for n := max(1, count-len(v.Always)); n > 0; n-- {
			out = append(out, g.storeItem(g.D.idx(v.Sometimes[g.random(len(v.Sometimes))]), lvl))
		}
	}
	return out
}

// BuyPrice is what a store charges (the identified value).
func BuyPrice(it *Item) int { return it.IValue }

// WirtPrice ports S_BBuyEnter: the boy's price_adjust on the item's value (1.09: half again, Hellfire: a quarter off).
func (d *Data) WirtPrice(it *Item) int {
	adj := d.Vendors["boy"].PriceAdjust
	if adj[1] == 0 {
		return it.IValue
	}
	return it.IValue + adj[0]*(it.IValue/adj[1])
}

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

// SmithBuys ports 1.09's SmithSellOk.
func (d *Data) SmithBuys(it *Item) bool {
	return it.Type != ITypeMisc && it.Type != ITypeGold && it.Type != ITypeStaff && it.BaseIdx != IdxLazStaff
}

// HellfireSmithBuys ports Hellfire's SmithSellOk: also oils and staves without a spell, but no quest items.
func (d *Data) HellfireSmithBuys(it *Item) bool {
	switch {
	case IsOil(it.MiscID):
		return true
	case it.Type == ITypeMisc, it.Type == ITypeGold, it.Type == ITypeStaff && it.Spell != 0, it.Class == "ICLASS_QUEST":
		return false
	}
	return it.BaseIdx != IdxLazStaff
}

// WitchBuys ports 1.09's WitchSellOk.
func (d *Data) WitchBuys(it *Item) bool {
	return (it.Type == ITypeMisc || it.Type == ITypeStaff) && !isQuestItem(it)
}

// HellfireWitchBuys ports Hellfire's WitchSellOk: no oils or quest items, and only staves with a spell.
func (d *Data) HellfireWitchBuys(it *Item) bool {
	ok := it.Type == ITypeMisc && !IsOil(it.MiscID) && it.Class != "ICLASS_QUEST" || it.Type == ITypeStaff && it.Spell != 0
	return ok && !isQuestItem(it)
}

// CanRepair ports SmithRepairOk.
func CanRepair(it *Item) bool {
	if it.MaxDur <= 0 || it.Durability == it.MaxDur || it.Type == ITypeMisc || it.Type == ITypeGold {
		return false
	}
	// AddStoreHoldRepair leaves out an identified magic item whose repair would cost nothing
	return !(it.Magical != 0 && it.Identified && repairValue(it) == 0)
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
	v := max(repairValue(it), 1)
	if v > 1 {
		v >>= 1
	}
	return v
}

func repairValue(it *Item) int {
	value := it.Value
	if it.Magical != 0 && it.Identified {
		value = 30 * it.IValue / 100
	}
	return 100 * (it.MaxDur - it.Durability) / it.MaxDur * value / 100
}

// CanRecharge ports WitchRechargeOk; with recharge_uniques on the witch's record, uniques with charges too (Hellfire).
func (d *Data) CanRecharge(it *Item) bool {
	if d.Vendors["witch"].RechargeUniques && (it.MiscID == "IMISC_UNIQUE" || it.MiscID == "IMISC_STAFF") && it.Charges < it.MaxCharges {
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
