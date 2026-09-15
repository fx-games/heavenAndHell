package d1

import "math/rand"

// Player classes (PC_).
const (
	Warrior = iota
	Rogue
	Sorcerer
)

var ClassNames = [3]string{"Warrior", "Rogue", "Sorcerer"}

// player.cpp tables.
var (
	strengthTbl  = [3]int{30, 20, 15}
	magicTbl     = [3]int{10, 15, 35}
	dexterityTbl = [3]int{20, 30, 15}
	vitalityTbl  = [3]int{25, 20, 20}
	toBlkTbl     = [3]int{30, 20, 10}
	MaxStats     = [3][4]int{{250, 50, 60, 100}, {55, 70, 250, 80}, {45, 250, 85, 80}}
	ExpLvls      = [51]int{0, 2000, 4620, 8040, 12489, 18258, 25712, 35309, 47622, 63364, 83419, 108879, 141086,
		181683, 231075, 313656, 424067, 571190, 766569, 1025154, 1366227, 1814568, 2401895, 3168651, 4166200,
		5459523, 7130496, 9281874, 12042092, 15571031, 20066900, 25774405, 32994399, 42095202, 53525811,
		67831218, 85670061, 107834823, 135274799, 169122009, 210720231, 261657253, 323800420, 399335440,
		490808349, 601170414, 733825617, 892680222, 1082908612, 1310707109, 1583495809}
	attackFrames = [3]int{16, 18, 16} // PlrGFXAnimLens[class][1]
	hitFrames    = [3]int{6, 7, 8}    // PlrGFXAnimLens[class][6]
)

// HitRecoveryTicks ports PM_DoGotHit: the hit animation, one to three frames shorter with the recovery affixes.
func HitRecoveryTicks(class int, flags uint32) int {
	switch {
	case flags&ISplFastestRecover != 0:
		return hitFrames[class] - 3
	case flags&ISplFasterRecover != 0:
		return hitFrames[class] - 2
	case flags&ISplFastRecover != 0:
		return hitFrames[class] - 1
	}
	return hitFrames[class]
}

// Attributes (ATTRIB_).
const (
	AttrStr = iota
	AttrMag
	AttrDex
	AttrVit
)

// Hero is D1's PlayerStruct minus graphics and multiplayer. HP/mana use D1's 1/64 fixed point.
type Hero struct {
	Class                              int
	BaseStr, BaseMag, BaseDex, BaseVit int
	Level, Exp, StatPts                int
	HPBase, MaxHPBase                  int
	ManaBase, MaxManaBase              int
	BaseToBlk                          int
	SpellLevels                        [37]int
}

// NewHero ports CreatePlayer.
func NewHero(class int) *Hero {
	h := &Hero{Class: class, BaseStr: strengthTbl[class], BaseMag: magicTbl[class], BaseDex: dexterityTbl[class],
		BaseVit: vitalityTbl[class], Level: 1, BaseToBlk: toBlkTbl[class]}
	hp := (h.BaseVit + 10) << 6
	mana := h.BaseMag << 6
	switch class {
	case Warrior:
		hp *= 2
	case Rogue:
		hp += hp >> 1
		mana += mana >> 1
	case Sorcerer:
		mana *= 2
	}
	h.HPBase, h.MaxHPBase, h.ManaBase, h.MaxManaBase = hp, hp, mana, mana
	if class == Sorcerer {
		h.SpellLevels[SplFirebolt] = 2
	}
	return h
}

// Sheet is what CalcPlrItemVals derives from the hero and their equipment.
type Sheet struct {
	Str, Mag, Dex, Vit                 int
	MinDam, MaxDam                     int
	AC, BonusAC, BonusDam, BonusToHit  int
	BonusDamMod, DamageMod, GetHit     int
	EnAc, SplLvlAdd                    int
	FR, LR, MR                         int
	Flags                              uint32
	LightRad                           int
	MaxHP, MaxMana                     int // 1/64 fixed point, includes item bonuses
	FMinDam, FMaxDam, LMinDam, LMaxDam int
	Block                              bool
	WeaponType                         int // ITYPE of the equipped weapon, ITypeMisc = none
	AttackFrames                       int
	Active                             []bool // per equipped item: requirements met (_iStatFlag)
}

// Sheet ports CalcSelfItems + CalcPlrItemVals: items whose Str/Mag/Dex requirements aren't met do nothing.
func (h *Hero) Sheet(equipped []*Item) Sheet {
	active := make([]bool, len(equipped))
	for i, it := range equipped {
		active[i] = it.MaxDur == 0 || it.Durability > 0 // broken items do nothing until repaired (decision 9)
	}
	for changed := true; changed; {
		changed = false
		str, mag, dex := h.BaseStr, h.BaseMag, h.BaseDex
		for i, it := range equipped {
			if active[i] && it.Identified { // CalcSelfItems counts only identified bonuses
				str, mag, dex = str+it.PLStr, mag+it.PLMag, dex+it.PLDex
			}
		}
		for i, it := range equipped {
			if active[i] && (str < it.MinStr || mag < it.MinMag || dex < it.MinDex) {
				active[i] = false
				changed = true
			}
		}
	}

	s := Sheet{LightRad: 10, Active: active}
	var sadd, madd, dadd, vadd, ihp, imana int
	hasShieldOnly := false
	for i, it := range equipped {
		if !active[i] {
			continue
		}
		s.MinDam += it.MinDam
		s.MaxDam += it.MaxDam
		s.AC += it.AC
		if it.Type == ITypeShield {
			hasShieldOnly = true
			s.Block = true
		}
		if it.Class == "ICLASS_WEAPON" {
			s.WeaponType = it.Type
		}
		if it.Magical != 0 && !it.Identified { // unidentified magic items give only their base damage and armor
			continue
		}
		s.BonusDam += it.PLDam
		s.BonusToHit += it.PLToHit
		if it.PLAC != 0 {
			s.BonusAC += max(it.PLAC*it.AC/100, 1)
		}
		s.Flags |= it.Flags
		sadd += it.PLStr
		madd += it.PLMag
		dadd += it.PLDex
		vadd += it.PLVit
		s.FR += it.PLFR
		s.LR += it.PLLR
		s.MR += it.PLMR
		s.BonusDamMod += it.PLDamMod
		s.GetHit += it.PLGetHit
		s.LightRad += it.PLLight
		ihp += it.PLHP << 6
		imana += it.PLMana << 6
		s.SplLvlAdd += it.SplLvlAdd
		s.EnAc += it.PLEnAc
		s.FMinDam += it.FMinDam
		s.FMaxDam += it.FMaxDam
		s.LMinDam += it.LMinDam
		s.LMaxDam += it.LMaxDam
	}
	if s.MinDam == 0 && s.MaxDam == 0 {
		s.MinDam, s.MaxDam = 1, 1
		if hasShieldOnly {
			s.MaxDam = 3
		}
	}
	s.LightRad = min(max(s.LightRad, 2), 15)
	s.Str = max(sadd+h.BaseStr, 0)
	s.Mag = max(madd+h.BaseMag, 0)
	s.Dex = max(dadd+h.BaseDex, 0)
	s.Vit = max(vadd+h.BaseVit, 0)
	if h.Class == Rogue {
		s.DamageMod = h.Level * (s.Str + s.Dex) / 200
	} else {
		s.DamageMod = h.Level * s.Str / 100
	}
	if s.Flags&ISplAllResZero != 0 {
		s.FR, s.LR, s.MR = 0, 0, 0
	}
	s.FR, s.LR, s.MR = min(s.FR, 75), min(s.LR, 75), min(s.MR, 75)
	switch h.Class {
	case Warrior:
		vadd *= 2
	case Rogue:
		vadd += vadd >> 1
		madd += madd >> 1
	case Sorcerer:
		madd *= 2
	}
	s.MaxHP = ihp + vadd<<6 + h.MaxHPBase
	s.MaxMana = imana + madd<<6 + h.MaxManaBase

	// SetPlrAnims: attack length by class and weapon, then PM_DoAttack's frame skips.
	s.AttackFrames = attackFrames[h.Class]
	switch {
	case h.Class == Warrior && s.WeaponType == ITypeAxe:
		s.AttackFrames = 20
	case h.Class == Rogue && s.WeaponType == ITypeAxe:
		s.AttackFrames = 22
	case h.Class == Rogue && s.WeaponType == ITypeBow:
		s.AttackFrames = 12
	case h.Class == Sorcerer && (s.WeaponType == ITypeMisc && !hasShieldOnly || s.WeaponType == ITypeBow):
		s.AttackFrames = 20
	case h.Class == Sorcerer && s.WeaponType == ITypeAxe:
		s.AttackFrames = 24
	}
	for flag, skip := range map[uint32]int{ISplQuickAttack: 1, ISplFastAttack: 2, ISplFasterAttack: 3, ISplFastestAttack: 4} {
		if s.Flags&flag != 0 {
			s.AttackFrames -= skip
		}
	}
	return s
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// MeleeHitChance ports PlrHitMonst's to-hit.
func (h *Hero) MeleeHitChance(s Sheet, monAC int) int {
	hper := s.EnAc + s.Dex>>1 - monAC + h.Level + 50
	if h.Class == Warrior {
		hper += 20
	}
	return clamp(hper+s.BonusToHit, 5, 95)
}

// MeleeDamage ports PlrHitMonst's damage roll (whole points). monClass is MC_UNDEAD/MC_DEMON/MC_ANIMAL.
func (h *Hero) MeleeDamage(rng *rand.Rand, s Sheet, monClass string) int {
	d := s.MinDam + rng.Intn(max(s.MaxDam-s.MinDam+1, 1))
	dam := s.BonusDamMod + s.DamageMod + d*s.BonusDam/100 + d
	if h.Class == Warrior && rng.Intn(100) < h.Level {
		dam *= 2
	}
	switch {
	case monClass == "MC_UNDEAD" && s.WeaponType == ITypeSword, monClass == "MC_ANIMAL" && s.WeaponType == ITypeMace:
		dam -= dam >> 1
	case monClass == "MC_UNDEAD" && s.WeaponType == ITypeMace, monClass == "MC_ANIMAL" && s.WeaponType == ITypeSword:
		dam += dam >> 1
	}
	if s.Flags&ISpl3xDamVsDemons != 0 && monClass == "MC_DEMON" {
		dam *= 3
	}
	return dam
}

// HitLeech ports PlrHitMonst's life/mana steal; dam in whole points, returns gains in 1/64.
func (h *Hero) HitLeech(rng *rand.Rand, s Sheet, dam int) (hp, mana int) {
	d := dam << 6
	if s.Flags&ISplRndStealLife != 0 {
		hp += rng.Intn(max(d>>3, 1))
	}
	if s.Flags&ISplNoMana == 0 {
		if s.Flags&ISplStealMana3 != 0 {
			mana = 3 * d / 100
		}
		if s.Flags&ISplStealMana5 != 0 {
			mana = 5 * d / 100
		}
	}
	if s.Flags&ISplStealLife3 != 0 {
		hp += 3 * d / 100
	}
	if s.Flags&ISplStealLife5 != 0 {
		hp += 5 * d / 100
	}
	return
}

// ArrowHitChance ports MonsterMHit for physical missiles; dist in cells.
func (h *Hero) ArrowHitChance(s Sheet, monAC, dist int) int {
	hper := h.Level + 50 - monAC + s.EnAc + s.BonusToHit + s.Dex - dist*dist>>1
	switch h.Class {
	case Rogue:
		hper += 20
	case Warrior:
		hper += 10
	}
	return clamp(hper, 5, 95)
}

// ArrowDamage ports MonsterMHit's physical damage (whole points).
func (h *Hero) ArrowDamage(rng *rand.Rand, s Sheet) int {
	return h.ArrowBlastDamage(rng, s, s.MinDam, s.MaxDam)
}

// ArrowBlastDamage is MonsterMHit's damage for any arrow missile (mType 0), also the burst of fire and lightning
// arrows: the roll plus the damage bonuses.
func (h *Hero) ArrowBlastDamage(rng *rand.Rand, s Sheet, mind, maxd int) int {
	dam := mind + rng.Intn(max(maxd-mind+1, 1))
	dam += s.BonusDamMod + dam*s.BonusDam/100
	if h.Class == Rogue {
		return dam + s.DamageMod
	}
	return dam + s.DamageMod>>1
}

func dlvlHitFloor(hper, dlvl int) int {
	switch dlvl {
	case 14:
		return max(hper, 20)
	case 15:
		return max(hper, 25)
	case 16:
		return max(hper, 30)
	}
	return hper
}

// MonsterMeleeHitChance ports M_TryH2HHit's to-hit against the hero.
func (h *Hero) MonsterMeleeHitChance(s Sheet, monHit, monLevel, dlvl int) int {
	tac := s.AC + s.BonusAC + s.Dex/5
	return dlvlHitFloor(max(monHit+30+(monLevel-h.Level)<<1-tac, 15), dlvl)
}

// MonsterArrowHitChance ports PlayerMHit for physical missiles; dist in cells.
func (h *Hero) MonsterArrowHitChance(s Sheet, monHit, monLevel, dist, dlvl int) int {
	tac := s.AC + s.BonusAC + s.Dex/5
	return dlvlHitFloor(max(monHit+30+(monLevel-h.Level)<<1-tac-dist<<1, 10), dlvl)
}

// MonsterSpellHitChance ports PlayerMHit for magic missiles; dist in cells.
func (h *Hero) MonsterSpellHitChance(monLevel, dist, dlvl int) int {
	return dlvlHitFloor(max(40+(monLevel-h.Level)<<1-dist<<1, 10), dlvl)
}

// MissileDamage64 ports PlayerMHit's damage roll in 1/64. With shift (lingering missiles hitting every tick)
// mind..maxd are already 1/64; D1 still adds the whole -damage affix unshifted and hits for at least 1 HP.
func MissileDamage64(rng *rand.Rand, s Sheet, mind, maxd int, shift bool) int {
	if shift {
		return max(rnd(rng, maxd-mind+1)+mind+s.GetHit, 64)
	}
	return MonsterDamage(rng, s, mind, maxd)
}

// TrapHitChance ports PlayerMHit for a missile from no monster (a trap): 100 − AC/2 − 2·dist for arrows, a flat
// 40 for magic; dist in cells.
func TrapHitChance(s Sheet, magic bool, dist, dlvl int) int {
	if magic {
		return dlvlHitFloor(40, dlvl)
	}
	return dlvlHitFloor(max(100-(s.AC+s.BonusAC+s.Dex/5)>>1-dist<<1, 10), dlvl)
}

// TrapDamage64 ports PlayerMHit's damage roll for a trap's missile, in 1/64: like MissileDamage64, but halved by
// ISPL_ABSHALFTRAP before the -damage affix is added.
func TrapDamage64(rng *rand.Rand, s Sheet, mind, maxd int, shift bool) int {
	var dam, getHit int
	if shift {
		dam, getHit = rnd(rng, maxd-mind+1)+mind, s.GetHit
	} else {
		dam, getHit = rng.Intn((maxd-mind+1)<<6)+mind<<6, s.GetHit<<6
	}
	if s.Flags&ISplAbsHalfTrap != 0 {
		dam >>= 1
	}
	return max(dam+getHit, 64)
}

// BlockChance ports the block roll; 0 without a shield.
func (h *Hero) BlockChance(s Sheet, monLevel int) int {
	if !s.Block {
		return 0
	}
	return clamp(h.BaseToBlk+s.Dex-(monLevel-h.Level)<<1, 0, 100)
}

// MonsterDamage ports the monster damage roll against the hero, in 1/64 (minimum 1 HP).
func MonsterDamage(rng *rand.Rand, s Sheet, minDam, maxDam int) int {
	dam := rng.Intn((maxDam-minDam+1)<<6) + minDam<<6 + s.GetHit<<6
	return max(dam, 64)
}

// AddExperience ports AddPlrExperience (single player); returns levels gained.
func (h *Hero) AddExperience(monLevel, exp int) int {
	e := int(float64(exp) * (float64(monLevel-h.Level)/10 + 1))
	h.Exp += max(e, 0)
	gained := 0
	if h.Exp >= ExpLvls[49] {
		for h.Level < 50 {
			h.nextLevel()
			gained++
		}
		return gained
	}
	l := 0
	for h.Exp >= ExpLvls[l] {
		l++
	}
	for ; h.Level < l; gained++ {
		h.nextLevel()
	}
	return gained
}

func (h *Hero) statDiff() int {
	m := MaxStats[h.Class]
	return m[AttrStr] - h.BaseStr + m[AttrMag] - h.BaseMag + m[AttrDex] - h.BaseDex + m[AttrVit] - h.BaseVit
}

// nextLevel ports NextPlrLevel (single player: +1/64 extra per level, full heal and mana).
func (h *Hero) nextLevel() {
	h.Level++
	if d := h.statDiff(); d < 5 {
		h.StatPts = d
	} else {
		h.StatPts += 5
	}
	hp, mana := 128+1, 128+1
	if h.Class == Sorcerer {
		hp = 64 + 1
	}
	if h.Class == Warrior {
		mana = 64 + 1
	}
	h.MaxHPBase += hp
	h.HPBase = h.MaxHPBase
	h.MaxManaBase += mana
	h.ManaBase = h.MaxManaBase // ponytail: D1 skips this refill with ISPL_NOMANA equipped; caller zeroes mana then.
}

// AddBaseStat ports ModifyPlr{Str,Mag,Dex,Vit}: raises a base attribute (capped) and returns HP/mana gained in 1/64.
func (h *Hero) AddBaseStat(attr, n int) (hp, mana int) {
	base := [4]*int{&h.BaseStr, &h.BaseMag, &h.BaseDex, &h.BaseVit}[attr]
	n = max(min(n, MaxStats[h.Class][attr]-*base), -*base) // ModifyPlr* caps at MaxStats, CheckStats floors at 0
	if n == 0 {
		return 0, 0
	}
	*base += n
	switch attr {
	case AttrMag:
		mana = n << 6
		if h.Class == Sorcerer {
			mana *= 2
		}
		h.MaxManaBase += mana
		h.ManaBase += mana
	case AttrVit:
		hp = n << 6
		if h.Class == Warrior {
			hp *= 2
		}
		h.MaxHPBase += hp
		h.HPBase += hp
	}
	return
}

// SpendStatPoint spends one level-up point on an attribute; false if none left or capped.
func (h *Hero) SpendStatPoint(attr int) bool {
	base := [4]int{h.BaseStr, h.BaseMag, h.BaseDex, h.BaseVit}[attr]
	if h.StatPts <= 0 || base >= MaxStats[h.Class][attr] {
		return false
	}
	h.AddBaseStat(attr, 1)
	h.StatPts--
	return true
}

// potionAmount is UseItem's IMISC_HEAL/IMISC_MANA formula in 1/64 for roll in [0, maxV>>8); bonus is the class
// that gets double.
func (h *Hero) potionAmount(maxV, roll, bonus int) int {
	l := (maxV>>8>>1 + roll) << 6
	switch h.Class {
	case bonus:
		l *= 2
	case Rogue:
		l += l >> 1
	}
	return l
}

// PotionHeal ports UseItem's IMISC_HEAL amount in 1/64; maxHP in 1/64.
func (h *Hero) PotionHeal(rng *rand.Rand, maxHP int) int {
	return h.potionAmount(maxHP, rng.Intn(max(maxHP>>8, 1)), Warrior)
}

// PotionMana ports UseItem's IMISC_MANA amount in 1/64; maxMana in 1/64.
func (h *Hero) PotionMana(rng *rand.Rand, maxMana int) int {
	return h.potionAmount(maxMana, rng.Intn(max(maxMana>>8, 1)), Sorcerer)
}

// PotionHealRange and PotionManaRange are the lowest and highest PotionHeal/PotionMana rolls, in 1/64.
func (h *Hero) PotionHealRange(maxHP int) (int, int) {
	return h.potionAmount(maxHP, 0, Warrior), h.potionAmount(maxHP, max(maxHP>>8, 1)-1, Warrior)
}

func (h *Hero) PotionManaRange(maxMana int) (int, int) {
	return h.potionAmount(maxMana, 0, Sorcerer), h.potionAmount(maxMana, max(maxMana>>8, 1)-1, Sorcerer)
}

// SpeedTiers is the calibration knob from decision 13: D1 animation lengths snap to these speed factors.
var SpeedTiers = []float64{0.5, 1, 1.5, 2}

// Reference animation lengths in D1 ticks for one turn: a walk step and a warrior's sword swing.
const (
	RefWalkTicks   = 8
	RefAttackTicks = 16
)

// SpeedTier maps an animation length to the nearest speed tier (ref/ticks).
func SpeedTier(refTicks, ticks int) float64 {
	if ticks <= 0 {
		return 1
	}
	r := float64(refTicks) / float64(ticks)
	best := SpeedTiers[0]
	for _, t := range SpeedTiers {
		if abs(t-r) < abs(best-r) {
			best = t
		}
	}
	return best
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
