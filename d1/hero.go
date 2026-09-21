package d1

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"strconv"
	"strings"
)

// Hero archetypes, in D1's PC_ order: the shapes the engine's formulas know how to play. They are NOT a roster -
// how many classes a game HAS is content, and is the number of rows in the mod's hero.rec. A mod's Nth class
// plays as the Nth archetype, so a three-class game gets warrior/rogue/sorcerer and names them as it likes.
const (
	Warrior = iota
	Rogue
	Sorcerer
	Monk
	Bard
	Barbarian
	NumArchetypes
)

// ClassNames are the class names, in the mod's own order; filled by Load from assets/data/hero.rec.
var ClassNames []string

// Class flags (PlayerClassFlag), straight off each class' attributes.tsv.
const (
	ClassCriticalStrike    = 1 << iota // clvl% of melee swings hit twice
	ClassTrapSense                     // trapped doors and chests read as trapped
	ClassIronSkin                      // +clvl/4 armor class
	ClassNaturalResistance             // +clvl to every resistance
	ClassDualWield                     // may hold a weapon in the off hand (not modelled, see D1_PLAN.md)
)

// HasClassFlag reports a PlayerClassFlag of the class, e.g. ClassTrapSense.
func HasClassFlag(class int, flag uint32) bool { return classAttr[class].Flags&flag != 0 }

// fp is devilutionX's parseFixed6: the attributes.tsv decimals are 1/64 multipliers.
func fp(v float64) int { return int(v * 64) }

// classAttributes is one row of devilutionX's txtdata/classes/<class>/attributes.tsv and animations.tsv, which
// reproduce D1 1.09's player.cpp tables and carry Hellfire's three extra classes. Everything the old per-class
// `switch h.Class` branches used to hardcode now comes from here.
type classAttributes struct {
	Str, Mag, Dex, Vit                  int
	MaxStr, MaxMag, MaxDex, MaxVit      int
	ToBlk                               int
	AdjLife, AdjMana                    int // flat part of the starting pool, 1/64
	LvlLife, LvlMana                    int // gained per character level, 1/64
	ChrLife, ChrMana                    int // gained per point of Vitality / Magic, 1/64
	ItmLife, ItmMana                    int // multiplier on Vitality / Magic from items, 1/64
	PotionLife, PotionMana              int // multiplier on a potion's roll (RestorePartialLife/Mana), 1/64
	SpellLife                           int // multiplier on the Healing spell's roll (AddHealing), 1/64
	ManaCost                            int // multiplier on a spell's mana cost (GetManaAmount), 1/64
	MagicToHit, MeleeToHit, RangedToHit int
	AttackFrames                        map[int]int // attack animation length by weapon ITYPE, key 0 = bare-handed and the default
	SpellFrames, HitFrames              int
	Flags                               uint32
	StartSpells                         map[int]int // spell levels a new hero of this class knows
}

// classAttr is filled by Load from the class section of assets/data/hero.rec.
var classAttr []classAttributes

// MaxStats is the attribute cap per class and ATTRIB_ (CharacterAttribute), derived from classAttr by Load.
var MaxStats [][4]int

// ExpLvls is the experience total each character level needs; filled by Load.
var ExpLvls [51]int

// itypeByName and classFlagByName name the columns of hero.rec that are enums rather than numbers, so the file
// reads as words instead of as the bit patterns and ITYPE_ numbers they compile to.
var itypeByName = map[string]int{"default": 0, "sword": ITypeSword, "axe": ITypeAxe, "bow": ITypeBow,
	"mace": ITypeMace, "staff": ITypeStaff}

var classFlagByName = map[string]uint32{"critical_strike": ClassCriticalStrike, "trap_sense": ClassTrapSense,
	"iron_skin": ClassIronSkin, "natural_resistance": ClassNaturalResistance, "dual_wield": ClassDualWield}

// heroTablesLoaded records whether the active mod supplied hero.rec. It is NOT optional in the way the xl_*
// tables are: those describe features a mod may not have, while this one is the hero's own numbers, and a game
// with no hero numbers has no hero. It is tolerated only so that the rest of a mod - its monsters, items,
// vendors and levels - can be loaded and worked on, and HeroTablesLoaded exists so nothing quietly builds a
// hero out of a table of zeroes. Zeroed class tables have silently broken this engine once already.
var heroTablesLoaded bool

// HeroTablesLoaded reports whether the active mod declared the hero's per-class numbers.
func HeroTablesLoaded() bool { return heroTablesLoaded }

func loadHeroTables(fsys fs.FS) error {
	heroTablesLoaded = false
	recs, err := readRecMulti(fsys, "hero.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	classes := recs["class"]
	// The roster LENGTH is the mod's to choose. The engine's only stake in it is that every row lands on an
	// archetype whose formulas exist, which is what caps it at NumArchetypes.
	if len(classes) == 0 || len(classes) > NumArchetypes {
		return fmt.Errorf("hero.rec: %d classes, want 1..%d", len(classes), NumArchetypes)
	}
	ClassNames = make([]string, len(classes))
	classAttr = make([]classAttributes, len(classes))
	MaxStats = make([][4]int, len(classes))
	SpellFrames = make([]int, len(classes))
	for i, r := range classes {
		m := r.ToMap("")
		// fp6 reads one of the 1/64 fixed-point columns, which the file carries as a plain decimal.
		fp6 := func(key string) int { return fp(m.GetFloatOrDefault(key, 0)) }
		a := classAttributes{
			Str: m.GetIntOrDefault("str", 0), Mag: m.GetIntOrDefault("mag", 0),
			Dex: m.GetIntOrDefault("dex", 0), Vit: m.GetIntOrDefault("vit", 0),
			MaxStr: m.GetIntOrDefault("max_str", 0), MaxMag: m.GetIntOrDefault("max_mag", 0),
			MaxDex: m.GetIntOrDefault("max_dex", 0), MaxVit: m.GetIntOrDefault("max_vit", 0),
			ToBlk:   m.GetIntOrDefault("to_blk", 0),
			AdjLife: fp6("adj_life"), AdjMana: fp6("adj_mana"),
			LvlLife: fp6("lvl_life"), LvlMana: fp6("lvl_mana"),
			ChrLife: fp6("chr_life"), ChrMana: fp6("chr_mana"),
			ItmLife: fp6("itm_life"), ItmMana: fp6("itm_mana"),
			PotionLife: fp6("potion_life"), PotionMana: fp6("potion_mana"),
			SpellLife: fp6("spell_life"), ManaCost: fp6("mana_cost"),
			MagicToHit: m.GetIntOrDefault("magic_to_hit", 0), MeleeToHit: m.GetIntOrDefault("melee_to_hit", 0),
			RangedToHit:  m.GetIntOrDefault("ranged_to_hit", 0),
			AttackFrames: map[int]int{}, StartSpells: map[int]int{},
			SpellFrames: m.GetIntOrDefault("spell_frames", 0), HitFrames: m.GetIntOrDefault("hit_frames", 0),
		}
		for _, f := range strings.Fields(m.GetStringOrDefault("attack_frames", "")) {
			name, frames, ok := strings.Cut(f, "=")
			itype, known := itypeByName[name]
			if !ok || !known {
				return fmt.Errorf("hero.rec: %s: bad attack_frames %q", ClassNames[i], f)
			}
			n, convErr := strconv.Atoi(frames)
			if convErr != nil {
				return fmt.Errorf("hero.rec: %s: attack_frames %q: %w", ClassNames[i], f, convErr)
			}
			a.AttackFrames[itype] = n
		}
		for _, f := range strings.Fields(m.GetStringOrDefault("start_spells", "")) {
			name, lvl, ok := strings.Cut(f, "=")
			spell, known := spellIDByName[name]
			n, convErr := strconv.Atoi(lvl)
			if !ok || !known || convErr != nil {
				return fmt.Errorf("hero.rec: %s: bad start_spells %q", m.GetStringOrDefault("name", ""), f)
			}
			a.StartSpells[spell] = n
		}
		for _, f := range strings.Fields(m.GetStringOrDefault("flags", "")) {
			bit, ok := classFlagByName[f]
			if !ok {
				return fmt.Errorf("hero.rec: unknown class flag %q", f)
			}
			a.Flags |= bit
		}
		ClassNames[i] = m.GetStringOrDefault("name", "")
		classAttr[i] = a
		MaxStats[i] = [4]int{a.MaxStr, a.MaxMag, a.MaxDex, a.MaxVit}
		SpellFrames[i] = a.SpellFrames
	}
	var exp []int
	for _, r := range recs["experience"] {
		for _, line := range fieldsNamed(r, "exp") {
			for _, f := range strings.Fields(line) {
				n, convErr := strconv.Atoi(f)
				if convErr != nil {
					return fmt.Errorf("hero.rec: exp %q: %w", f, convErr)
				}
				exp = append(exp, n)
			}
		}
	}
	if len(exp) != len(ExpLvls) {
		return fmt.Errorf("hero.rec: %d experience levels, want %d", len(exp), len(ExpLvls))
	}
	copy(ExpLvls[:], exp)
	heroTablesLoaded = true
	return nil
}

// HitRecoveryTicks ports PM_DoGotHit: the hit animation, one to three frames shorter with the recovery affixes.
func HitRecoveryTicks(class int, flags uint32) int {
	switch {
	case flags&ISplFastestRecover != 0:
		return classAttr[class].HitFrames - 3
	case flags&ISplFasterRecover != 0:
		return classAttr[class].HitFrames - 2
	case flags&ISplFastRecover != 0:
		return classAttr[class].HitFrames - 1
	}
	return classAttr[class].HitFrames
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
	SpellLevels                        [NumSpells]int
	Rage                               int      // the Barbarian's Rage: turns of fury left, or minus the turns of hangover left
	SkillOrder                         []string // skill bar order the player dragged (skill names); not D1's
	Reflections                        int      // Hellfire's Reflect (wReflections): melee blows left to turn back, until the next level
}

// NewHero ports CreatePlayer, with calculateBaseLife/Mana in place of D1's per-class multipliers.
func NewHero(class int) *Hero {
	a := classAttr[class]
	h := &Hero{Class: class, BaseStr: a.Str, BaseMag: a.Mag, BaseDex: a.Dex, BaseVit: a.Vit, Level: 1, BaseToBlk: a.ToBlk}
	hp := a.AdjLife + a.LvlLife*h.Level + a.ChrLife*h.BaseVit
	mana := a.AdjMana + a.LvlMana*h.Level + a.ChrMana*h.BaseMag
	h.HPBase, h.MaxHPBase, h.ManaBase, h.MaxManaBase = hp, hp, mana, mana
	for spell, lvl := range a.StartSpells {
		h.SpellLevels[spell] = lvl
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
	DamAcFlags                         uint32 // Hellfire's ItemSpecialEffectHf, see IHf* in oils.go
	LightRad                           int
	MaxHP, MaxMana                     int // 1/64 fixed point, includes item bonuses
	FMinDam, FMaxDam, LMinDam, LMaxDam int
	Block                              bool
	WeaponType                         int    // ITYPE of the equipped weapon, ITypeMisc = none (the off hand's wins)
	WeaponMask                         uint32 // one bit per ITYPE held, for the hands a dual wielder fills separately
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

	a := classAttr[h.Class]
	s := Sheet{LightRad: 10, Active: active}
	var sadd, madd, dadd, vadd, ihp, imana int
	var shieldAC, weapons, twoHanders int
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
			shieldAC = it.AC
		}
		if it.Class == "ICLASS_WEAPON" {
			s.WeaponType = it.Type
			s.WeaponMask |= 1 << it.Type
			weapons++
			if it.Loc == "ILOC_TWOHAND" {
				twoHanders++
			}
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
		s.DamAcFlags |= it.DamAcFlags
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
	bareHanded := s.WeaponType == ITypeMisc && !hasShieldOnly
	if s.MinDam == 0 && s.MaxDam == 0 { // CalcPlrDamage: bare fists, which the Monk trains
		s.MinDam, s.MaxDam = 1, 1
		if hasShieldOnly {
			s.MaxDam = 3
		}
		if h.Class == Monk {
			s.MinDam, s.MaxDam = max(s.MinDam, h.Level/2), max(s.MaxDam, h.Level)
		}
	}
	if h.Class == Monk { // CalcPlrBlockFlag: the Monk parries with a staff, a single one-hander or bare hands
		switch {
		case s.Holds(ITypeStaff):
			s.Block = true
			s.Flags |= ISplFastBlock
		case weapons == 0 || weapons == 1 && twoHanders == 0:
			s.Block = true
		}
	}
	s.LightRad = min(max(s.LightRad, 2), 15)
	// CalcPlrPrimaryStats: Rage lends the Barbarian attributes while it burns and takes the same back afterwards.
	if h.Rage != 0 {
		n := h.Level
		if h.Rage < 0 {
			n = -n
		}
		sadd += 2 * n
		dadd += n + n/2
		vadd += 2 * n
	}
	s.Str = max(sadd+h.BaseStr, 0)
	s.Mag = max(madd+h.BaseMag, 0)
	s.Dex = max(dadd+h.BaseDex, 0)
	s.Vit = max(vadd+h.BaseVit, 0)
	strDex := h.Level * (s.Str + s.Dex)
	switch h.Class { // CalcPlrDamage's damage bonus, which every Hellfire class weighs differently
	case Rogue:
		s.DamageMod = strDex / 200
	case Monk:
		if s.Holds(ITypeStaff) || bareHanded {
			s.DamageMod = strDex / 150
		} else {
			s.DamageMod = strDex / 300
		}
	case Bard:
		switch {
		case s.Holds(ITypeSword):
			s.DamageMod = strDex / 150
		case s.Holds(ITypeBow):
			s.DamageMod = strDex / 250
		default:
			s.DamageMod = h.Level * s.Str / 100
		}
	case Barbarian:
		switch {
		case s.Holds(ITypeAxe) || s.Holds(ITypeMace):
			s.DamageMod = h.Level * s.Str / 75
		case s.Holds(ITypeBow):
			s.DamageMod = h.Level * s.Str / 300
		default:
			s.DamageMod = h.Level * s.Str / 100
		}
		if hasShieldOnly { // a Barbarian hides behind only half a shield
			s.AC -= shieldAC / 2
		} else if !s.Holds(ITypeStaff) && !s.Holds(ITypeBow) {
			s.DamageMod += h.Level * s.Vit / 100
		}
	default:
		s.DamageMod = h.Level * s.Str / 100
	}
	if a.Flags&ClassIronSkin != 0 {
		s.BonusAC += h.Level / 4
	}
	if a.Flags&ClassNaturalResistance != 0 { // CalcPlrResistances
		s.FR, s.LR, s.MR = s.FR+h.Level, s.LR+h.Level, s.MR+h.Level
	}
	if h.Rage < 0 { // the Rage hangover strips the same again
		s.FR, s.LR, s.MR = s.FR-h.Level, s.LR-h.Level, s.MR-h.Level
	}
	if s.Flags&ISplAllResZero != 0 {
		s.FR, s.LR, s.MR = 0, 0, 0
	}
	s.FR, s.LR, s.MR = clamp(s.FR, 0, 75), clamp(s.LR, 0, 75), clamp(s.MR, 0, 75)
	s.MaxHP = ihp + (vadd*a.ItmLife>>6)<<6 + h.MaxHPBase
	s.MaxMana = imana + (madd*a.ItmMana>>6)<<6 + h.MaxManaBase
	// Acolyte's Amulet and Gladiator's Ring trade one pool for the other (IPL_MANATOLIFE / IPL_LIFETOMANA)
	if s.DamAcFlags&IHfManaToLife != 0 {
		p := (h.MaxManaBase >> 6) * 50 / 100 << 6
		s.MaxMana, s.MaxHP = s.MaxMana-p, s.MaxHP+p
	}
	if s.DamAcFlags&IHfLifeToMana != 0 {
		p := (h.MaxHPBase >> 6) * 40 / 100 << 6
		s.MaxHP, s.MaxMana = s.MaxHP-p, s.MaxMana+p
	}

	// SetPlrAnims: attack length by class and weapon. Only the Sorcerer's bare
	// hands are slower than the same hands behind a shield.
	s.AttackFrames = a.AttackFrames[0]
	if n, ok := a.AttackFrames[s.WeaponType]; ok {
		s.AttackFrames = n
	} else if h.Class == Sorcerer && hasShieldOnly && s.WeaponType == ITypeMisc {
		s.AttackFrames = 16
	}
	return s
}

// MeleeFrames is AttackFrames after PM_DoAttack's frame skips. PM_DoRangeAttack has none, so bows don't speed up.
func (s Sheet) MeleeFrames() int {
	n := s.AttackFrames
	for flag, skip := range map[uint32]int{ISplQuickAttack: 1, ISplFastAttack: 2, ISplFasterAttack: 3, ISplFastestAttack: 4} {
		if s.Flags&flag != 0 {
			n -= skip
		}
	}
	return n
}

// Holds reports an ITYPE in either hand, like devilutionX's Player::isHoldingItem.
func (s Sheet) Holds(itype int) bool { return s.WeaponMask&(1<<itype) != 0 }

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// MeleeHitChance ports PlrHitMonst's to-hit.
func (h *Hero) MeleeHitChance(s Sheet, monAC int) int {
	hper := s.EnAc + s.Dex>>1 - monAC + h.Level + classAttr[h.Class].MeleeToHit
	return clamp(hper+s.BonusToHit, 5, 95)
}

// MeleeDamage ports PlrHitMonst's damage roll (whole points). monClass is MC_UNDEAD/MC_DEMON/MC_ANIMAL.
func (h *Hero) MeleeDamage(rng *rand.Rand, s Sheet, monClass string) int {
	d := s.MinDam + rng.Intn(max(s.MaxDam-s.MinDam+1, 1))
	crit := classAttr[h.Class].Flags&ClassCriticalStrike != 0
	return h.meleeDamageOf(s, d, crit && rng.Intn(100) < h.Level, monClass)
}

// MeleeDamageRange is MeleeDamage's lowest roll without and highest roll with the warrior's double damage.
func (h *Hero) MeleeDamageRange(s Sheet, monClass string) (lo, hi int) {
	crit := classAttr[h.Class].Flags&ClassCriticalStrike != 0 && h.Level > 0
	return h.meleeDamageOf(s, s.MinDam, false, monClass), h.meleeDamageOf(s, s.MaxDam, crit, monClass)
}

func (h *Hero) meleeDamageOf(s Sheet, d int, double bool, monClass string) int {
	dam := s.BonusDamMod + s.DamageMod + d*s.BonusDam/100 + d
	if double {
		dam *= 2
	}
	switch s.WeaponVsMonsterClass(monClass) {
	case -50:
		dam -= dam >> 1
	case 50:
		dam += dam >> 1
	}
	if s.Flags&ISpl3xDamVsDemons != 0 && monClass == "MC_DEMON" {
		dam *= 3
	}
	return dam
}

// WeaponVsMonsterClass is what the weapon in hand does to melee damage against a monster class, as a percentage
// (+50, -50 or 0): PlrHitMonst's phanditype rule - swords cut flesh and maces crush bone. A sword in either hand
// wins over a mace. Nothing else, staves and axes included, cares what it is hitting.
func (s Sheet) WeaponVsMonsterClass(monClass string) int {
	sword, mace := s.Holds(ITypeSword), !s.Holds(ITypeSword) && s.Holds(ITypeMace)
	switch {
	case monClass == "MC_UNDEAD" && sword, monClass == "MC_ANIMAL" && mace:
		return -50
	case monClass == "MC_UNDEAD" && mace, monClass == "MC_ANIMAL" && sword:
		return 50
	}
	return 0
}

// CriticalStrikeChance is the percentage of melee swings that land twice (MeleeDamage's crit roll): the
// character's level, for the classes that have it, and 0 for the rest.
func (h *Hero) CriticalStrikeChance() int {
	if !HasClassFlag(h.Class, ClassCriticalStrike) {
		return 0
	}
	return min(h.Level, 100)
}

// FullMissileDamageBonus reports a class that adds its whole Str/Dex damage bonus to missiles instead of half of
// it (arrowDamageOf).
func FullMissileDamageBonus(class int) bool { return class == Rogue || class == Bard }

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
	if s.Flags&ISplStealLife5 != 0 { // the 5% overrides the 3%, as with mana
		hp += 5 * d / 100
	} else if s.Flags&ISplStealLife3 != 0 {
		hp += 3 * d / 100
	}
	return
}

// ArrowHitChance ports MonsterMHit for physical missiles; dist in cells.
func (h *Hero) ArrowHitChance(s Sheet, monAC, dist int) int {
	hper := h.Level + classAttr[h.Class].RangedToHit - monAC + s.EnAc + s.BonusToHit + s.Dex - dist*dist>>1
	return clamp(hper, 5, 95)
}

// ArrowDamage ports MonsterMHit's physical damage (whole points).
func (h *Hero) ArrowDamage(rng *rand.Rand, s Sheet) int {
	return h.ArrowBlastDamage(rng, s, s.MinDam, s.MaxDam)
}

// ArrowBlastDamage is MonsterMHit's damage for any arrow missile (mType 0), also the burst of fire and lightning
// arrows: the roll plus the damage bonuses.
func (h *Hero) ArrowBlastDamage(rng *rand.Rand, s Sheet, mind, maxd int) int {
	return h.arrowDamageOf(s, mind+rng.Intn(max(maxd-mind+1, 1)))
}

func (h *Hero) arrowDamageOf(s Sheet, dam int) int {
	dam += s.BonusDamMod + dam*s.BonusDam/100
	if h.Class == Rogue || h.Class == Bard {
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

// ArrowDamageRange is ArrowBlastDamage's lowest and highest roll.
func (h *Hero) ArrowDamageRange(s Sheet, mind, maxd int) (lo, hi int) {
	return h.arrowDamageOf(s, mind), h.arrowDamageOf(s, maxd)
}

// ArmorVs is the hero's armour class against one monster class: Bone Chain Armor and Demon Plate Armor turn it
// up against the undead and against demons (M_TryH2HHit's ACAgainstUndead/ACAgainstDemons).
func (s Sheet) ArmorVs(monClass string) int {
	ac := s.AC + s.BonusAC + s.Dex/5
	if s.DamAcFlags&IHfACVsDemons != 0 && monClass == "MC_DEMON" {
		ac += 40
	}
	if s.DamAcFlags&IHfACVsUndead != 0 && monClass == "MC_UNDEAD" {
		ac += 20
	}
	return ac
}

// MonsterMeleeHitChance ports M_TryH2HHit's to-hit against the hero.
func (h *Hero) MonsterMeleeHitChance(s Sheet, monHit, monLevel, dlvl int, monClass string) int {
	return dlvlHitFloor(max(monHit+30+(monLevel-h.Level)<<1-s.ArmorVs(monClass), 15), dlvl)
}

// MonsterArrowHitChance ports PlayerMHit for physical missiles; dist in cells.
func (h *Hero) MonsterArrowHitChance(s Sheet, monHit, monLevel, dist, dlvl int) int {
	tac := s.AC + s.BonusAC + s.Dex/5
	return dlvlHitFloor(max(monHit+30+(monLevel-h.Level)<<1-tac-dist<<1, 10), dlvl)
}

// MonsterSpellHitChance ports PlayerMHit for magic missiles. Their distance term is always 0: only MI_Arrow and
// MI_LArrow ever advance _midist.
func (h *Hero) MonsterSpellHitChance(monLevel, dlvl int) int {
	return dlvlHitFloor(max(40+(monLevel-h.Level)<<1, 10), dlvl)
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
		dam, getHit = rnd(rng, (maxd-mind+1)<<6)+mind<<6, s.GetHit<<6
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
	dam := rnd(rng, (maxDam-minDam+1)<<6) + minDam<<6 + s.GetHit<<6
	return max(dam, 64)
}

// AddExperience ports AddPlrExperience (single player); returns levels gained.
func (h *Hero) AddExperience(monLevel, exp int) int {
	e := int(float64(exp) * (float64(monLevel-h.Level)/10 + 1))
	h.Exp = min(h.Exp+max(e, 0), 2000000000) // MAXEXP
	gained := 0
	if h.Exp >= ExpLvls[49] { // level 50 outright, without NextPlrLevel's points, life and mana
		gained, h.Level = 50-h.Level, 50
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
	a := classAttr[h.Class]
	hp, mana := a.LvlLife+1, a.LvlMana+1 // single player: one extra 1/64 of each per level
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
		mana = n * classAttr[h.Class].ChrMana
		h.MaxManaBase += mana
		h.ManaBase += mana
	case AttrVit:
		hp = n * classAttr[h.Class].ChrLife
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

// potionAmount is RestorePartialLife/RestorePartialMana in 1/64 for roll in [0, maxV>>8); factor is the class'
// 1/64 multiplier on the roll.
func potionAmount(maxV, roll, factor int) int {
	return (maxV>>8>>1 + roll) << 6 * factor >> 6
}

// PotionHeal ports UseItem's IMISC_HEAL amount in 1/64; maxHP in 1/64.
func (h *Hero) PotionHeal(rng *rand.Rand, maxHP int) int {
	return potionAmount(maxHP, rng.Intn(max(maxHP>>8, 1)), classAttr[h.Class].PotionLife)
}

// PotionMana ports UseItem's IMISC_MANA amount in 1/64; maxMana in 1/64.
func (h *Hero) PotionMana(rng *rand.Rand, maxMana int) int {
	return potionAmount(maxMana, rng.Intn(max(maxMana>>8, 1)), classAttr[h.Class].PotionMana)
}

// PotionHealRange and PotionManaRange are the lowest and highest PotionHeal/PotionMana rolls, in 1/64.
func (h *Hero) PotionHealRange(maxHP int) (int, int) {
	f := classAttr[h.Class].PotionLife
	return potionAmount(maxHP, 0, f), potionAmount(maxHP, max(maxHP>>8, 1)-1, f)
}

func (h *Hero) PotionManaRange(maxMana int) (int, int) {
	f := classAttr[h.Class].PotionMana
	return potionAmount(maxMana, 0, f), potionAmount(maxMana, max(maxMana>>8, 1)-1, f)
}

// RefWalkTicks is D1's walk step in ticks, one 10-AUT turn: the D1 clock (decision 13).
const RefWalkTicks = 8
