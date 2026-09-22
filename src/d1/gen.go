package d1

import (
	"fmt"
	"strconv"
	"strings"
)

// ISPL_ item special-effect bits.
const (
	ISplInfravision    = 0x00000001
	ISplRndStealLife   = 0x00000002
	ISplRndArrowVel    = 0x00000004
	ISplFireArrows     = 0x00000008
	ISplFireDam        = 0x00000010
	ISplLightDam       = 0x00000020
	ISplDrainLife      = 0x00000040
	ISplNoHealPlr      = 0x00000100
	ISplKnockback      = 0x00000800
	ISplNoHealMon      = 0x00001000
	ISplStealMana3     = 0x00002000
	ISplStealMana5     = 0x00004000
	ISplStealLife3     = 0x00008000
	ISplStealLife5     = 0x00010000
	ISplQuickAttack    = 0x00020000
	ISplFastAttack     = 0x00040000
	ISplFasterAttack   = 0x00080000
	ISplFastestAttack  = 0x00100000
	ISplFastRecover    = 0x00200000
	ISplFasterRecover  = 0x00400000
	ISplFastestRecover = 0x00800000
	ISplFastBlock      = 0x01000000
	ISplLightArrows    = 0x02000000
	ISplMultArrows     = 0x10000000
	ISplThorns         = 0x04000000
	ISplNoMana         = 0x08000000
	ISplAbsHalfTrap    = 0x10000000
	ISpl3xDamVsDemons  = 0x40000000
	ISplAllResZero     = 0x80000000
)

// Item is a rolled D1 item (ItemStruct minus graphics). HP/Mana bonuses are whole points, not D1's <<6.
type Item struct {
	BaseIdx                            int
	Name, IName                        string // unidentified / identified name
	Type, Curs                         int
	Class, Loc, MiscID                 string
	Spell                              int // SPL_ index, 0 = none
	MinDam, MaxDam, AC                 int
	Flags                              uint32
	Magical                            int // 0 normal, 1 magic, 2 unique
	Identified                         bool
	Value, IValue                      int
	vAdd1, vMult1, vAdd2, vMult2       int
	PLDam, PLToHit, PLAC               int
	PLStr, PLMag, PLDex, PLVit         int
	Charges, MaxCharges                int
	Durability, MaxDur                 int
	MinStr, MinMag, MinDex             int
	PLFR, PLLR, PLMR                   int
	PLDamMod, PLGetHit, PLLight        int
	SplLvlAdd                          int
	FMinDam, FMaxDam, LMinDam, LMaxDam int
	PLEnAc, PLMana, PLHP               int
	DamAcFlags                         uint32 // Hellfire's ItemSpecialEffectHf
	PrePower, SufPower                 string
	Uid                                int // unique index, -1 = none
	Seed                               int32
	CreateInfo                         int // _iCreateInfo: item level and CF_ flags, to recreate the item
}

// _iCreateInfo flags.
const (
	CFLevel    = 0x3F
	CFOnlyGood = 0x40
	CFUper15   = 0x80
	CFUper1    = 0x100
	CFUnique   = 0x200
	CFPregen   = 0x8000
)

// Generator rolls D1 loot with D1's RNG. UniqueDropped is game state (single player: each unique drops once) and must
// be saved.
type Generator struct {
	D             *Data
	R             *Rng
	UniqueDropped []bool
}

func NewGenerator(d *Data, seed int32) *Generator {
	g := &Generator{D: d, R: &Rng{}, UniqueDropped: make([]bool, len(d.UniqueItems))}
	g.R.SetSeed(seed)
	return g
}

func (g *Generator) random(v int) int { return g.R.Random(v) }

// itemAttrs ports GetItemAttrs. dlvl is the dungeon level (for gold), lvl the item level.
func (g *Generator) itemAttrs(idx, lvl, dlvl int, hell bool) *Item {
	b := g.D.Items[idx]
	it := &Item{
		BaseIdx: idx, Name: b.Name, IName: b.Name, Type: b.Type, Curs: b.Curs, Class: b.Class, Loc: b.Loc, MiscID: b.MiscID,
		Spell: g.spellIdx(b.Spell), MinDam: b.MinDam, MaxDam: b.MaxDam, AC: g.random(b.MaxAC-b.MinAC+1) + b.MinAC,
		Flags: g.isplFlags(b.Flags), Value: b.Value, IValue: b.Value, Durability: b.Durability, MaxDur: b.Durability,
		MinStr: b.MinStr, MinMag: b.MinMag, MinDex: b.MinDex, Uid: -1,
	}
	if it.MiscID == "IMISC_BOOK" {
		g.bookSpell(it, lvl)
	}
	if it.MiscID == "IMISC_OILOF" { // a plain Oil settles into one of the ten as it drops
		g.getOilType(it, lvl)
	}
	for _, m := range b.Modifiers {
		g.recordPower(it, m, lvl)
	}
	if it.Type == ITypeGold {
		v := 5*dlvl + g.random(10*dlvl)
		if hell {
			v += v >> 3
		}
		it.Value = min(v, 5000)
	}
	return it
}

// recordPower applies an equip_modifier a record names (H&H's "strength", "light_radius(2)") as the item power that
// means the same, rolled at item level lvl the way an affix is. A modifier with an argument uses it as the value.
func (g *Generator) recordPower(it *Item, modifier string, lvl int) {
	name, arg, hasArg := strings.Cut(strings.TrimSuffix(modifier, ")"), "(")
	lo, hi := 1, max(lvl, 1)
	if hasArg {
		if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
			lo, hi = n, n
		}
	}
	power := map[string]string{
		"strength": "IPL_STR", "dexterity": "IPL_DEX", "intelligence": "IPL_MAG", "health": "IPL_LIFE", "mana": "IPL_MANA",
		"damage": "IPL_DAMMOD", "armor_class": "IPL_ACP", "light_radius": "IPL_LIGHT", "attack_speed": "IPL_FASTATTACK",
		"steal_health": "IPL_STEALLIFE", "steal_mana": "IPL_STEALMANA",
	}[name]
	switch power {
	case "":
		return // speed and move_speed are no item power: the item carries them as stat modifiers (applyItemRecord)
	case "IPL_ACP":
		lo, hi = 10*lo, 10*hi // percent
	case "IPL_FASTATTACK":
		lo, hi = 1, 1
	}
	g.savePower(it, power, lo, hi, 0, 0, 1)
}

func (g *Generator) spellIdx(id string) int {
	for i, s := range g.D.Spells {
		if s.ID == id {
			return i
		}
	}
	return 0
}

func (g *Generator) isplFlags(s string) uint32 {
	if s == "ISPL_RNDSTEALLIFE" {
		return ISplRndStealLife
	}
	return 0
}

// nextSpell walks spells like GetBookSpell/GetStaffSpell, skipping multiplayer-only ones. It wraps at MAX_SPELLS,
// the size of the mod's spell table: 37 for Diablo 1.09, 52 for a table with Hellfire's spells.
func (g *Generator) nextSpell(ok func(SpellData) bool) int {
	n := g.D.SpellCount
	rv := g.random(n) + 1
	s, bs := 1, 0
	for rv > 0 {
		if ok(g.D.Spells[s]) {
			rv--
			bs = s
		}
		s++
		if s == 32 { // SPL_RESURRECT
			s = 33
		}
		if s == 34 { // SPL_HEALOTHER
			s = 35
		}
		if s == n {
			s = 1
		}
	}
	return bs
}

func (g *Generator) bookSpell(it *Item, lvl int) {
	lvl = max(lvl, 1)
	bs := g.nextSpell(func(s SpellData) bool { return s.BookLvl != -1 && lvl >= s.BookLvl })
	sp := g.D.Spells[bs]
	// The base item is named "Book of" and the spell completes it. Diablo's own table wrote the separator as a
	// trailing space inside the name; a .rec field cannot carry one, and a name ending in a space was never
	// content anyway -- it was the concatenation, which belongs here.
	it.Name += " " + sp.Name
	it.IName += " " + sp.Name
	it.Spell = bs
	it.MinMag = sp.MinInt
	it.Value += sp.BookCost
	it.IValue += sp.BookCost
}

func (g *Generator) rndPL(p1, p2 int) int { return p1 + g.random(p2-p1+1) }

func plVal(pv, p1, p2, minv, maxv int) int {
	if p1 == p2 || minv == maxv {
		return minv
	}
	return minv + (maxv-minv)*(100*(pv-p1)/(p2-p1))/100
}

// savePower ports SaveItemPower.
func (g *Generator) savePower(it *Item, power string, p1, p2, minv, maxv, multv int) {
	r := g.rndPL(p1, p2)
	switch power {
	case "IPL_TOHIT":
		it.PLToHit += r
	case "IPL_TOHIT_CURSE":
		it.PLToHit -= r
	case "IPL_DAMP":
		it.PLDam += r
	case "IPL_DAMP_CURSE":
		it.PLDam -= r
	case "IPL_DOPPELGANGER", "IPL_TOHIT_DAMP": // Doppelganger's hits are IPL_TOHIT_DAMP that may split the monster
		if power == "IPL_DOPPELGANGER" {
			it.DamAcFlags |= IHfDoppelganger
		}
		r = g.rndPL(p1, p2)
		it.PLDam += r
		r2 := 0
		switch p1 {
		case 20:
			r2 = g.rndPL(1, 5)
		case 36:
			r2 = g.rndPL(6, 10)
		case 51:
			r2 = g.rndPL(11, 15)
		case 66:
			r2 = g.rndPL(16, 20)
		case 81:
			r2 = g.rndPL(21, 30)
		case 96:
			r2 = g.rndPL(31, 40)
		case 111:
			r2 = g.rndPL(41, 50)
		case 126:
			r2 = g.rndPL(51, 75)
		case 151:
			r2 = g.rndPL(76, 100)
		}
		it.PLToHit += r2
	case "IPL_TOHIT_DAMP_CURSE":
		it.PLDam -= r
		r2 := 0
		switch p1 {
		case 25:
			r2 = g.rndPL(1, 5)
		case 50:
			r2 = g.rndPL(6, 10)
		}
		it.PLToHit -= r2
	case "IPL_ACP":
		it.PLAC += r
	case "IPL_ACP_CURSE":
		it.PLAC -= r
	case "IPL_SETAC":
		it.AC = r
	case "IPL_AC_CURSE":
		it.AC -= r
	case "IPL_FIRERES":
		it.PLFR += r
	case "IPL_LIGHTRES":
		it.PLLR += r
	case "IPL_MAGICRES":
		it.PLMR += r
	case "IPL_ALLRES":
		it.PLFR = max(it.PLFR+r, 0)
		it.PLLR = max(it.PLLR+r, 0)
		it.PLMR = max(it.PLMR+r, 0)
	case "IPL_SPLLVLADD":
		it.SplLvlAdd = r
	case "IPL_CHARGES":
		it.Charges *= p1
		it.MaxCharges = it.Charges
	case "IPL_SPELL":
		it.Spell = p1
		it.Charges = p1
		it.MaxCharges = p2
	case "IPL_FIREDAM":
		it.Flags |= ISplFireDam
		it.FMinDam, it.FMaxDam = p1, p2
	case "IPL_LIGHTDAM":
		it.Flags |= ISplLightDam
		it.LMinDam, it.LMaxDam = p1, p2
	case "IPL_STR":
		it.PLStr += r
	case "IPL_STR_CURSE":
		it.PLStr -= r
	case "IPL_MAG":
		it.PLMag += r
	case "IPL_MAG_CURSE":
		it.PLMag -= r
	case "IPL_DEX":
		it.PLDex += r
	case "IPL_DEX_CURSE":
		it.PLDex -= r
	case "IPL_VIT":
		it.PLVit += r
	case "IPL_VIT_CURSE":
		it.PLVit -= r
	case "IPL_ATTRIBS":
		it.PLStr += r
		it.PLMag += r
		it.PLDex += r
		it.PLVit += r
	case "IPL_ATTRIBS_CURSE":
		it.PLStr -= r
		it.PLMag -= r
		it.PLDex -= r
		it.PLVit -= r
	case "IPL_GETHIT_CURSE":
		it.PLGetHit += r
	case "IPL_GETHIT":
		it.PLGetHit -= r
	case "IPL_LIFE":
		it.PLHP += r
	case "IPL_LIFE_CURSE":
		it.PLHP -= r
	case "IPL_MANA":
		it.PLMana += r
	case "IPL_MANA_CURSE":
		it.PLMana -= r
	case "IPL_DUR":
		r2 := r * it.MaxDur / 100
		it.MaxDur += r2
		it.Durability += r2
	case "IPL_DUR_CURSE":
		it.MaxDur = max(it.MaxDur-r*it.MaxDur/100, 1)
		it.Durability = it.MaxDur
	case "IPL_INDESTRUCTIBLE":
		it.Durability, it.MaxDur = 255, 255
	case "IPL_LIGHT":
		it.PLLight += p1
	case "IPL_LIGHT_CURSE":
		it.PLLight -= p1
	case "IPL_FIRE_ARROWS":
		it.Flags |= ISplFireArrows
		it.FMinDam, it.FMaxDam = p1, p2
	case "IPL_LIGHT_ARROWS":
		it.Flags |= ISplLightArrows
		it.LMinDam, it.LMaxDam = p1, p2
	case "IPL_THORNS":
		it.Flags |= ISplThorns
	case "IPL_NOMANA":
		it.Flags |= ISplNoMana
	case "IPL_NOHEALPLR":
		it.Flags |= ISplNoHealPlr
	case "IPL_ABSHALFTRAP":
		it.Flags |= ISplAbsHalfTrap
	case "IPL_KNOCKBACK":
		it.Flags |= ISplKnockback
	case "IPL_3XDAMVDEM":
		it.Flags |= ISpl3xDamVsDemons
	case "IPL_ALLRESZERO":
		it.Flags |= ISplAllResZero
	case "IPL_NOHEALMON":
		it.Flags |= ISplNoHealMon
	case "IPL_STEALMANA":
		if p1 == 3 {
			it.Flags |= ISplStealMana3
		}
		if p1 == 5 {
			it.Flags |= ISplStealMana5
		}
	case "IPL_STEALLIFE":
		if p1 == 3 {
			it.Flags |= ISplStealLife3
		}
		if p1 == 5 {
			it.Flags |= ISplStealLife5
		}
	case "IPL_TARGAC":
		it.PLEnAc += r
	case "IPL_FASTATTACK":
		it.Flags |= [5]uint32{0, ISplQuickAttack, ISplFastAttack, ISplFasterAttack, ISplFastestAttack}[min(max(p1, 0), 4)]
	case "IPL_FASTRECOVER":
		it.Flags |= [4]uint32{0, ISplFastRecover, ISplFasterRecover, ISplFastestRecover}[min(max(p1, 0), 3)]
	case "IPL_FASTBLOCK":
		it.Flags |= ISplFastBlock
	case "IPL_DAMMOD":
		it.PLDamMod += r
	case "IPL_RNDARROWVEL":
		it.Flags |= ISplRndArrowVel
	case "IPL_SETDAM":
		it.MinDam, it.MaxDam = p1, p2
	case "IPL_SETDUR":
		it.Durability, it.MaxDur = p1, p1
	case "IPL_FASTSWING":
		it.Flags |= ISplFasterAttack
	case "IPL_ONEHAND":
		it.Loc = "ILOC_ONEHAND"
	case "IPL_DRAINLIFE":
		it.Flags |= ISplDrainLife
	case "IPL_RNDSTEALLIFE":
		it.Flags |= ISplRndStealLife
	case "IPL_INFRAVISION":
		it.Flags |= ISplInfravision
	case "IPL_NOMINSTR":
		it.MinStr = 0
	case "IPL_MULT_ARROWS":
		it.Flags |= ISplMultArrows
	case "IPL_FIREBALL": // Flambeau: arrows that burst like a fireball
		it.Flags |= ISplLightArrows | ISplFireArrows
		it.FMinDam, it.FMaxDam = p1, p2
		it.LMinDam, it.LMaxDam = 0, 0
	case "IPL_ADDACLIFE": // Blitzen: arrows that hit for the wearer's armour class
		it.Flags |= ISplLightArrows | ISplFireArrows
		it.FMinDam, it.FMaxDam = p1, p2
		it.LMinDam, it.LMaxDam = 1, 0
	case "IPL_ADDMANAAC": // Thunderclap: hits that hurt for the wearer's mana
		it.Flags |= ISplLightDam | ISplFireDam
		it.FMinDam, it.FMaxDam = p1, p2
		it.LMinDam, it.LMaxDam = 2, 0
	case "IPL_MANATOLIFE":
		it.DamAcFlags |= IHfManaToLife
	case "IPL_LIFETOMANA":
		it.DamAcFlags |= IHfLifeToMana
	case "IPL_ACDEMON":
		it.DamAcFlags |= IHfACVsDemons
	case "IPL_ACUNDEAD":
		it.DamAcFlags |= IHfACVsUndead
	case "IPL_DEVASTATION":
		it.DamAcFlags |= IHfDevastation
	case "IPL_PERIL":
		it.DamAcFlags |= IHfPeril
	case "IPL_JESTERS":
		it.DamAcFlags |= IHfJesters
	case "IPL_DECAY":
		it.DamAcFlags |= IHfDecay
		it.PLDam += r
	case "IPL_CRYSTALLINE": // as IPL_DUR_CURSE, with a huge damage bonus
		it.PLDam += 140 + 2*r
		it.MaxDur = max(it.MaxDur-r*it.MaxDur/100, 1)
		it.Durability = it.MaxDur
	case "IPL_INVCURS":
		it.Curs = p1
		// IPL_ADDACLIFE/ADDMANAAC/FIRERESCLVL aren't ported: in 1.09 only Hellfire's Blitzen and Thunderclap roll the
		// first two, and no item or affix rolls FIRERESCLVL.
	}
	if it.vAdd1 == 0 && it.vMult1 == 0 {
		it.vAdd1, it.vMult1 = plVal(r, p1, p2, minv, maxv), multv
	} else {
		it.vAdd2, it.vMult2 = plVal(r, p1, p2, minv, maxv), multv
	}
}

func (g *Generator) calcValue(it *Item) {
	v := it.vMult1 + it.vMult2
	if v > 0 {
		v *= it.Value
	}
	if v < 0 {
		v = it.Value / v
	}
	v += it.vAdd1 + it.vAdd2
	it.IValue = max(v, 1)
}

func (g *Generator) applyPrefix(it *Item, a Affix) {
	it.IName = a.Name + " " + it.IName
	it.Magical = 1
	g.savePower(it, a.Power, a.P1, a.P2, a.MinVal, a.MaxVal, a.MultVal)
	it.PrePower = a.Power
}

// itemPower ports GetItemPower.
func (g *Generator) itemPower(it *Item, minlvl, maxlvl, flgs int, onlygood bool) {
	pre, post := g.random(4), g.random(3)
	if pre != 0 && post == 0 {
		if g.random(2) != 0 {
			post = 1
		} else {
			pre = 0
		}
	}
	if !onlygood && g.random(3) != 0 {
		onlygood = true
	}
	goe := 0
	magical := false
	if pre == 0 {
		var l []int
		for j, a := range g.D.Prefixes {
			if a.IType&flgs != 0 && a.MinLvl >= minlvl && a.MinLvl <= maxlvl && (!onlygood || a.Ok) && (flgs != PLTStaff || a.Power != "IPL_CHARGES") {
				l = append(l, j)
				if a.Double {
					l = append(l, j)
				}
			}
		}
		if len(l) != 0 {
			a := g.D.Prefixes[l[g.random(len(l))]]
			g.applyPrefix(it, a)
			goe = a.GOE
			magical = true
		}
	}
	if post != 0 {
		var l []int
		for j, a := range g.D.Suffixes {
			if a.IType&flgs != 0 && a.MinLvl >= minlvl && a.MinLvl <= maxlvl && (goe|a.GOE) != 0x11 && (!onlygood || a.Ok) {
				l = append(l, j)
			}
		}
		if len(l) != 0 {
			a := g.D.Suffixes[l[g.random(len(l))]]
			it.IName = it.IName + " of " + a.Name
			it.Magical = 1
			g.savePower(it, a.Power, a.P1, a.P2, a.MinVal, a.MaxVal, a.MultVal)
			it.SufPower = a.Power
			magical = true
		}
	}
	if magical {
		g.calcValue(it)
	}
}

// staffSpell ports GetStaffSpell + GetStaffPower.
func (g *Generator) staffSpell(it *Item, lvl int, onlygood bool) {
	if g.random(4) == 0 { // 1.09: one staff in four rolls a power instead of a spell (Hellfire dropped this)
		g.itemPower(it, lvl>>1, lvl, PLTStaff, onlygood)
		return
	}
	l := max(lvl>>1, 1)
	bs := g.nextSpell(func(s SpellData) bool { return s.StaffLvl != -1 && l >= s.StaffLvl })
	sp := g.D.Spells[bs]
	it.Name = fmt.Sprintf("%s of %s", it.Name, sp.Name)
	it.IName = it.Name
	it.Spell = bs
	it.Charges = g.random(sp.StaffMax-sp.StaffMin+1) + sp.StaffMin
	it.MaxCharges = it.Charges
	it.MinMag = sp.MinInt
	v := it.Charges * sp.StaffCost / 5
	it.Value += v
	it.IValue += v
	if g.random(10) == 0 || onlygood {
		var cand []int
		for j, a := range g.D.Prefixes {
			if a.IType&PLTStaff != 0 && a.MinLvl <= lvl && (!onlygood || a.Ok) {
				cand = append(cand, j)
				if a.Double {
					cand = append(cand, j)
				}
			}
		}
		if len(cand) != 0 {
			g.applyPrefix(it, g.D.Prefixes[cand[g.random(len(cand))]])
		}
	}
	g.calcValue(it)
}

func (g *Generator) itemBonus(it *Item, minlvl, maxlvl int, onlygood bool) {
	g.itemBonusSpells(it, minlvl, maxlvl, onlygood, true)
}

// itemBonusSpells is Hellfire's GetItemBonus: without allowSpells a staff rolls a power instead of a spell.
func (g *Generator) itemBonusSpells(it *Item, minlvl, maxlvl int, onlygood, allowSpells bool) {
	if it.Class == "ICLASS_GOLD" {
		return
	}
	minlvl = min(minlvl, 25)
	switch it.Type {
	case ITypeSword, ITypeAxe, ITypeMace:
		g.itemPower(it, minlvl, maxlvl, PLTWeap, onlygood)
	case ITypeBow:
		g.itemPower(it, minlvl, maxlvl, PLTBow, onlygood)
	case ITypeShield:
		g.itemPower(it, minlvl, maxlvl, PLTShld, onlygood)
	case ITypeLArmor, ITypeHelm, ITypeMArmor, ITypeHArmor:
		g.itemPower(it, minlvl, maxlvl, PLTArmo, onlygood)
	case ITypeStaff:
		if allowSpells {
			g.staffSpell(it, maxlvl, onlygood)
		} else {
			g.itemPower(it, minlvl, maxlvl, PLTStaff, onlygood)
		}
	case ITypeRing, ITypeAmulet:
		g.itemPower(it, minlvl, maxlvl, PLTMisc, onlygood)
	}
}

// checkUnique ports CheckUnique, including D1's bug that the last eligible unique always wins.
func (g *Generator) checkUnique(it *Item, lvl, uper int, recreate bool) int {
	if g.random(100) > uper {
		return -1
	}
	j := -1
	for u, ud := range g.D.UniqueItems {
		if ud.Base == g.D.Items[it.BaseIdx].UniqueBase && lvl >= ud.MinLvl && (recreate || !g.UniqueDropped[u]) {
			j = u
		}
	}
	if j >= 0 {
		g.random(10) // D1 rolls and ignores it
	}
	return j
}

func (g *Generator) uniqueItem(it *Item, uid int) {
	g.UniqueDropped[uid] = true
	ud := g.D.UniqueItems[uid]
	for p := 0; p < min(max(ud.NumPL, 1), 6); p++ {
		g.savePower(it, ud.Powers[p].Power, ud.Powers[p].P1, ud.Powers[p].P2, 0, 0, 1)
	}
	it.IName = ud.Name
	it.IValue = ud.Value
	it.Uid = uid
	it.Magical = 2
}

func (g *Generator) rndDur(it *Item) {
	if it.Durability == 0 || it.Durability == 255 {
		return
	}
	it.Durability = g.random(it.MaxDur>>1) + (it.MaxDur >> 2) + 1
}

// setupAllItems ports SetupAllItems: it reseeds the RNG with the item's seed. lvl is the item level (monster level or
// 2×dlvl).
func (g *Generator) setupAllItems(idx int, seed int32, lvl, dlvl, uper int, onlygood, recreate, pregen, hell bool) *Item {
	g.R.SetSeed(seed)
	it := g.itemAttrs(idx, lvl>>1, dlvl, hell)
	it.Seed, it.CreateInfo = seed, lvl
	if pregen {
		it.CreateInfo |= CFPregen
	}
	if onlygood {
		it.CreateInfo |= CFOnlyGood
	}
	if uper == 15 {
		it.CreateInfo |= CFUper15
	} else if uper == 1 {
		it.CreateInfo |= CFUper1
	}
	if it.MiscID != "IMISC_UNIQUE" {
		iblvl := -1
		if g.random(100) <= 10 || g.random(100) <= lvl {
			iblvl = lvl
		}
		if iblvl == -1 && (it.MiscID == "IMISC_STAFF" || it.MiscID == "IMISC_RING" || it.MiscID == "IMISC_AMULET") {
			iblvl = lvl
		}
		if onlygood {
			iblvl = lvl
		}
		if uper == 15 {
			iblvl = lvl + 4
		}
		if iblvl != -1 {
			if uid := g.checkUnique(it, iblvl, uper, recreate); uid == -1 {
				g.itemBonus(it, iblvl>>1, iblvl, onlygood)
			} else {
				g.uniqueItem(it, uid)
				it.CreateInfo |= CFUnique
			}
		}
		if it.Magical != 2 {
			g.rndDur(it)
		}
	}
	it.Identified = it.Magical == 0
	return it
}

// rndItem ports RndItem: a monster's drop, 0 none, -1-uid a fixed unique, otherwise base index + 1.
func (g *Generator) rndItem(m *MonsterData, mlvl int) int {
	if m.Treasure&0x8000 != 0 {
		return -1 - m.Treasure&0xFFF
	}
	if m.Treasure&0x4000 != 0 || g.random(100) > 40 {
		return 0
	}
	if g.random(100) > 25 {
		return IdxGold + 1
	}
	var ril []int
	for i, b := range g.D.Items {
		if b.Drop == "IDROP_DOUBLE" && mlvl >= b.MinMLvl {
			ril = append(ril, i)
		}
		if b.Drop != "IDROP_NEVER" && mlvl >= b.MinMLvl {
			ril = append(ril, i)
		}
		if b.Spell == "SPL_RESURRECT" || b.Spell == "SPL_HEALOTHER" { // D1 drops the last entry even when these weren't added
			ril = ril[:len(ril)-1]
		}
	}
	return ril[g.random(len(ril))] + 1
}

// rndUItem ports RndUItem: a unique monster's or good container's base item (mlvl < 0: use 2×dlvl).
func (g *Generator) rndUItem(m *MonsterData, mlvl, dlvl int) int {
	if m != nil && m.Treasure&0x8000 != 0 {
		return -1 - m.Treasure&0xFFF
	}
	var ril []int
	for i, b := range g.D.Items {
		ok := b.Drop != "IDROP_NEVER"
		if m != nil && mlvl < b.MinMLvl || m == nil && 2*dlvl < b.MinMLvl {
			ok = false
		}
		if b.Type == ITypeMisc || b.Type == ITypeGold || b.Type == ITypeFood {
			ok = false
		}
		if b.MiscID == "IMISC_BOOK" {
			ok = true
		}
		if b.Spell == "SPL_RESURRECT" || b.Spell == "SPL_HEALOTHER" {
			ok = false
		}
		if ok {
			ril = append(ril, i)
		}
	}
	return ril[g.random(len(ril))]
}

// rndAllItems ports RndAllItems: gold three times in four, otherwise any droppable base item.
func (g *Generator) rndAllItems(dlvl int) int {
	if g.random(100) > 25 {
		return IdxGold
	}
	var ril []int
	for i, b := range g.D.Items {
		if b.Drop != "IDROP_NEVER" && 2*dlvl >= b.MinMLvl {
			ril = append(ril, i)
		}
		if b.Spell == "SPL_RESURRECT" || b.Spell == "SPL_HEALOTHER" {
			ril = ril[:len(ril)-1]
		}
	}
	return ril[g.random(len(ril))]
}

// rndTypeItems ports RndTypeItems (imisc "" = any).
func (g *Generator) rndTypeItems(itype int, imisc string, dlvl int) int {
	var ril []int
	for i, b := range g.D.Items {
		if b.Drop != "IDROP_NEVER" && dlvl<<1 >= b.MinMLvl && b.Type == itype && (imisc == "" || b.MiscID == imisc) {
			ril = append(ril, i)
		}
	}
	return ril[g.random(len(ril))]
}

func (g *Generator) uniqueIdxByBase(uid int) int {
	for i, b := range g.D.Items {
		if b.UniqueBase == g.D.UniqueItems[uid].Base {
			return i
		}
	}
	return 0
}

// SpawnUnique ports SpawnUnique's item: a specific unique (quest and boss drops).
func (g *Generator) SpawnUnique(uid, dlvl int) *Item {
	it := g.itemAttrs(g.uniqueIdxByBase(uid), dlvl, dlvl, false)
	g.uniqueItem(it, uid)
	it.Identified = false
	return it
}

// MonsterDrop ports SpawnItem's roll: nil for no drop. mlvl is the monster's level (a unique's is raised), which
// picks the base item; the item rolls at the monster type's level like D1.
func (g *Generator) MonsterDrop(m *MonsterData, mlvl int, unique bool, dlvl int, hell bool) *Item {
	var idx int
	onlygood := unique
	if unique {
		idx = g.rndUItem(m, mlvl, dlvl)
	} else {
		if idx = g.rndItem(m, mlvl); idx == 0 {
			return nil
		}
		if idx > 0 {
			idx--
		}
	}
	if idx < 0 {
		return g.SpawnUnique(-(idx + 1), dlvl)
	}
	uper := 1
	if unique {
		uper = 15
	}
	return g.setupAllItems(idx, g.R.Seed(), m.Level, dlvl, uper, onlygood, false, false, hell)
}

// FloorItem ports CreateRndItem's roll (containers, theme rooms).
func (g *Generator) FloorItem(dlvl int, onlygood, hell bool) *Item {
	var idx int
	if onlygood {
		idx = g.rndUItem(nil, 0, dlvl)
	} else {
		idx = g.rndAllItems(dlvl)
	}
	return g.setupAllItems(idx, g.R.Seed(), 2*dlvl, dlvl, 1, onlygood, false, false, hell)
}

// LevelMonsterTypes ports GetLevelMTypes' random selection (quest additions are handled by the caller).
func (g *Generator) LevelMonsterTypes(dlvl int) []int {
	var list []int
	for i, m := range g.D.Monsters {
		if i == D1MonsterCount-1 {
			continue
		}
		minl, maxl := 15*m.MinDLvl/30+1, 15*m.MaxDLvl/30+1
		if dlvl >= minl && dlvl <= maxl && m.Available() {
			list = append(list, i)
		}
	}
	var types []int
	budget := 4000
	for len(list) > 0 && len(types) < 16 {
		for i := 0; i < len(list); {
			if g.D.Monsters[list[i]].ImgSize > budget {
				list[i] = list[len(list)-1]
				list = list[:len(list)-1]
			} else {
				i++
			}
		}
		if len(list) == 0 {
			break
		}
		rv := g.random(len(list))
		types = append(types, list[rv])
		budget -= g.D.Monsters[list[rv]].ImgSize
		list[rv] = list[len(list)-1]
		list = list[:len(list)-1]
	}
	return types
}

const shrineThaumaturgic = 8

// RandomShrine ports FindValidShrine for the goat shrine / cauldron, which reroll their effect at operate time.
// Single-player only, so shrineAvail==2 (multiplayer-only) shrines never appear.
func (g *Generator) RandomShrine(dlvl int) int {
	for {
		rv := g.random(len(g.D.ShrineNames))
		if rv == shrineThaumaturgic || g.D.shrineAvail[rv] == 2 || dlvl > g.D.shrineMax[rv] {
			continue
		}
		return rv
	}
}

// MonsterHP ports InitMonster's HP roll, halved for single player like D1.
func (g *Generator) MonsterHP(m *MonsterData) int {
	hp := g.random(m.MaxHP()-m.MinHP()+1) + m.MinHP()
	if m.ID == "MT_DIABLO" {
		hp = 1666
	}
	if m.FullHP {
		return max(hp, 1)
	}
	return max(hp>>1, 1)
}

// GroupSize ports InitMonsters' pack size roll.
func (g *Generator) GroupSize(dlvl int) int {
	if dlvl != 1 && g.random(2) != 0 {
		if dlvl == 2 {
			return g.random(2) + 2
		}
		return g.random(3) + 3
	}
	return 1
}

// PlainItem ports SetPlrHandItem: a base item without random AC or affixes (starting gear).
func (g *Generator) PlainItem(idx int) *Item {
	b := g.D.Items[idx]
	it := &Item{BaseIdx: idx, Name: b.Name, IName: b.Name, Type: b.Type, Curs: b.Curs, Class: b.Class, Loc: b.Loc,
		MiscID: b.MiscID, Spell: g.spellIdx(b.Spell), MinDam: b.MinDam, MaxDam: b.MaxDam, AC: b.MinAC, Value: b.Value,
		IValue: b.Value, Durability: b.Durability, MaxDur: b.Durability, MinStr: b.MinStr, MinMag: b.MinMag,
		MinDex: b.MinDex, Uid: -1, Identified: true}
	if b.MiscID == "IMISC_STAFF" {
		it.Charges, it.MaxCharges = 40, 40
	}
	if it.MiscID == "IMISC_OILOF" { // a plain Oil never stays plain, whichever path made it
		g.getOilType(it, 1)
	}
	for _, m := range b.Modifiers { // what the record says it does (H&H's torch: light_radius(1)); D1's say nothing
		g.recordPower(it, m, 1)
	}
	return it
}

// TypeItem ports CreateTypeItem's roll: an item of an ITYPE (and IMISC, "" = any) at item level 2×dlvl.
func (g *Generator) TypeItem(dlvl, itype int, imisc string, onlygood bool) *Item {
	idx := IdxGold
	if itype != ITypeGold {
		idx = g.rndTypeItems(itype, imisc, dlvl)
	}
	return g.setupAllItems(idx, g.R.Seed(), 2*dlvl, dlvl, 1, onlygood, false, false, dlvl > 12)
}

// WeaponItem is a good weapon of any kind the mod has, at item level 2×dlvl: a quest reward. Nil if the mod has
// no weapon that deep.
func (g *Generator) WeaponItem(dlvl int) *Item {
	var ril []int
	for i, b := range g.D.Items {
		if b.Drop != "IDROP_NEVER" && dlvl<<1 >= b.MinMLvl && (b.Class == "ICLASS_WEAPON" || b.IsWeapon()) {
			ril = append(ril, i)
		}
	}
	if len(ril) == 0 {
		return nil
	}
	return g.setupAllItems(ril[g.random(len(ril))], g.R.Seed(), 2*dlvl, dlvl, 1, true, false, false, dlvl > 12)
}

// MagicItemWithCurs ports CreateMagicArmor/CreateMagicWeapon: magic items of type itype are rolled until one has
// graphic curs.
func (g *Generator) MagicItemWithCurs(dlvl, itype, curs int) *Item {
	idx := g.rndTypeItems(itype, "", dlvl)
	for {
		it := g.setupAllItems(idx, g.R.Seed(), 2*dlvl, dlvl, 1, true, false, true, dlvl > 12)
		if it.Curs == curs {
			return it
		}
		idx = g.rndTypeItems(itype, "", dlvl)
	}
}

// SpellBook ports CreateSpellBook: a book rolled until it teaches spell.
func (g *Generator) SpellBook(dlvl, spell int) *Item {
	idx := g.rndTypeItems(ITypeMisc, "IMISC_BOOK", dlvl)
	for {
		if it := g.setupAllItems(idx, g.R.Seed(), 2*dlvl, dlvl, 1, true, false, true, dlvl > 12); it.Spell == spell {
			return it
		}
	}
}

// UsefulItem ports SetupAllUseful: a healing or mana potion, sometimes a town portal scroll.
func (g *Generator) UsefulItem(dlvl int) *Item {
	seed := g.R.Seed()
	g.R.SetSeed(seed)
	idx := IdxMana
	if g.random(2) != 0 {
		idx = IdxHeal
	}
	if dlvl > 1 && g.random(3) == 0 {
		idx = IdxPortal
	}
	it := g.itemAttrs(idx, dlvl, dlvl, false)
	it.Seed, it.CreateInfo, it.Identified = seed, dlvl|CFUper15|CFUper1, true
	return it
}
