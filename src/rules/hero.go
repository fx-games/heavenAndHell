package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"strconv"
	"strings"

	"rpg/src/defs"
)

// D1 and Hellfire's classes, by internal_name. Only the tests name them: a class is its classes.rec record, and
// everything it does differently from another is a flag or field there, which any class may take.
const (
	Warrior   = "warrior"
	Rogue     = "rogue"
	Sorcerer  = "sorcerer"
	Monk      = "monk"
	Bard      = "bard"
	Barbarian = "barbarian"
)

// HasClassFlag reports a class flag of the class, e.g. defs.ClassTrapSense.
func (d *Data) HasClassFlag(class string, flag uint32) bool { return d.Class(class).HasFlag(flag) }

// SlainHeroReward reads classes.rec's slain_hero: "book <SPL_>" leaves a spell book, anything else is an
// items.rec internal_name whose item type and graphic pick the magic item OperateSlainHero rolls.
func (d *Data) SlainHeroReward(class string) (book int, item *ItemData) {
	reward := d.Class(class).SlainHero
	if spell, ok := strings.CutPrefix(reward, "book "); ok {
		book, _ := d.SpellIdx(strings.TrimSpace(spell))
		return book, nil
	}
	for i := range d.Items {
		if reward != "" && d.Items[i].InternalName == reward {
			return 0, &d.Items[i]
		}
	}
	return 0, nil
}

// ClassSkills are the class skills the engine implements; a class names one in classes.rec's skill.
var ClassSkills = map[string]int{"repair": SplRepair, "disarm": SplDisarm, "recharge": SplRecharge,
	"vision": SplSearch, "identify": SplIdentify, "rage": SplRage}

// Class is the class whose internal_name is id (classes.rec), the zero record for one the mod doesn't have.
func (d *Data) Class(id string) *defs.LoadoutDefinition {
	if c, ok := d.classes[id]; ok {
		return c
	}
	return &defs.LoadoutDefinition{}
}

// ClassByKey is the class whose key is key, "" for none.
func (d *Data) ClassByKey(key string) string {
	for id, c := range d.classes {
		if c.Key == key {
			return id
		}
	}
	return ""
}

// itypeByName and classFlagByName name the columns of classes.rec that are enums rather than numbers, so the file
// reads as words instead of as the bit patterns and ITYPE_ numbers they compile to.
var itypeByName = map[string]int{"default": 0, "sword": ITypeSword, "axe": ITypeAxe, "bow": ITypeBow,
	"mace": ITypeMace, "staff": ITypeStaff}

// HeroTablesLoaded reports whether the active mod declared the hero's per-class numbers.
func (d *Data) HeroTablesLoaded() bool { return d.heroTablesLoaded }

func loadHeroTables(fsys fs.FS, d *Data) error {
	d.heroTablesLoaded = false
	recs, err := readRecMulti(fsys, "hero.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// The roster is the mod's classes.rec, parsed by defs; what's left here is checking the names only the
	// hero's rules know.
	classes, err := defs.ReadClasses(fsys)
	if err != nil {
		return err
	}
	d.classes = make(map[string]*defs.LoadoutDefinition, len(classes))
	for i := range classes {
		c := &classes[i]
		for name := range c.StartSpells {
			if _, ok := d.SpellIdx(name); !ok {
				return fmt.Errorf("classes.rec: %s: unknown start spell %q", c.InternalName, name)
			}
		}
		if _, ok := ClassSkills[c.Skill]; c.Skill != "" && !ok {
			return fmt.Errorf("classes.rec: %s: unknown skill %q", c.InternalName, c.Skill)
		}
		d.classes[c.InternalName] = c
		if book, item := d.SlainHeroReward(c.InternalName); c.SlainHero != "" && book == 0 && item == nil {
			return fmt.Errorf("classes.rec: %s: slain_hero %q is neither an item nor \"book <SPL_>\"", c.InternalName, c.SlainHero)
		}
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
	if len(exp) != len(d.ExpLvls) {
		return fmt.Errorf("hero.rec: %d experience levels, want %d", len(exp), len(d.ExpLvls))
	}
	copy(d.ExpLvls[:], exp)
	d.WeaponBonuses = nil
	for i, r := range recs["weapon_bonus"] {
		m := r.ToMap("")
		weapon, versus := m.GetStringOrDefault("weapon", ""), m.GetStringOrDefault("versus", "")
		itype, ok := itypeByName[weapon]
		pct, convErr := strconv.Atoi(m.GetStringOrDefault("percent", ""))
		if !ok || weapon == "default" || versus == "" || convErr != nil {
			return fmt.Errorf("hero.rec: weapon_bonus %d: want weapon (sword, axe, bow, mace, staff), versus (a monster class) and percent", i+1)
		}
		d.WeaponBonuses = append(d.WeaponBonuses, WeaponBonus{Weapon: itype, Versus: versus, Percent: pct})
	}
	d.heroTablesLoaded = true
	return nil
}

// HitRecoveryTicks ports PM_DoGotHit: the hit animation, one to three frames shorter with the recovery affixes.
func (d *Data) HitRecoveryTicks(class string, flags uint32) int {
	switch {
	case flags&ISplFastestRecover != 0:
		return d.Class(class).HitFrames - 3
	case flags&ISplFasterRecover != 0:
		return d.Class(class).HitFrames - 2
	case flags&ISplFastRecover != 0:
		return d.Class(class).HitFrames - 1
	}
	return d.Class(class).HitFrames
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
	Class                              string `json:"-"` // the class's internal_name; a save keeps it beside the hero
	BaseStr, BaseMag, BaseDex, BaseVit int
	Level, Exp, StatPts                int
	SkillPts                           int // level-up points not yet put into a skill (classes.rec skill_points)
	HPBase, MaxHPBase                  int
	ManaBase, MaxManaBase              int
	BaseToBlk                          int
	SpellLevels                        []int    // by row of Data.Spells; saved by spell id (MarshalJSON)
	Rage                               int      // the Barbarian's Rage: turns of fury left, or minus the turns of hangover left
	SkillOrder                         []string // skill bar order the player dragged (skill names); not D1's
	Reflections                        int      // Hellfire's Reflect (wReflections): melee blows left to turn back, until the next level

	d           *Data          // the rules this hero plays by; a loaded hero gets them back through Data.Adopt
	savedSpells map[string]int // SpellLevels as a save names them, until Adopt finds their rows
}

// MarshalJSON saves the spell levels by spell id, so a save outlives the order of the spell table.
func (h Hero) MarshalJSON() ([]byte, error) {
	type plain Hero
	levels := map[string]int{}
	for spell, lvl := range h.SpellLevels {
		if lvl != 0 && h.d != nil && h.d.Spells[spell].ID != "" {
			levels[h.d.Spells[spell].ID] = lvl
		}
	}
	return json.Marshal(struct {
		plain
		SpellLevels map[string]int
	}{plain(h), levels})
}

// UnmarshalJSON reads what MarshalJSON wrote, or an older save's array of D1 spell levels by SPL_ number.
func (h *Hero) UnmarshalJSON(b []byte) error {
	type plain Hero
	var v struct {
		*plain
		SpellLevels json.RawMessage
	}
	v.plain = (*plain)(h)
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	h.SpellLevels, h.savedSpells = nil, nil
	if len(v.SpellLevels) > 0 && v.SpellLevels[0] == '[' {
		return json.Unmarshal(v.SpellLevels, &h.SpellLevels)
	}
	if len(v.SpellLevels) > 0 {
		return json.Unmarshal(v.SpellLevels, &h.savedSpells)
	}
	return nil
}

// Adopt binds a hero read back from a save to these rules: a spell the mod no longer has is forgotten.
func (d *Data) Adopt(h *Hero) {
	h.d = d
	levels := make([]int, len(d.Spells))
	copy(levels, h.SpellLevels)
	for id, lvl := range h.savedSpells {
		if spell, ok := d.SpellIdx(id); ok {
			levels[spell] = lvl
		}
	}
	h.SpellLevels, h.savedSpells = levels, nil
}

// Rules are the tables the hero plays by.
func (h *Hero) Rules() *Data { return h.d }

// NewHero ports CreatePlayer, with calculateBaseLife/Mana in place of D1's per-class multipliers.
func (d *Data) NewHero(class string) *Hero {
	a := d.Class(class)
	h := &Hero{d: d, Class: class, BaseStr: a.Str, BaseMag: a.Mag, BaseDex: a.Dex, BaseVit: a.Vit, Level: 1, BaseToBlk: a.ToBlk}
	hp := a.AdjLife + a.LvlLife*h.Level + a.ChrLife*h.BaseVit
	mana := a.AdjMana + a.LvlMana*h.Level + a.ChrMana*h.BaseMag
	h.HPBase, h.MaxHPBase, h.ManaBase, h.MaxManaBase = hp, hp, mana, mana
	h.SpellLevels = make([]int, len(d.Spells))
	for spell, lvl := range a.StartSpells {
		idx, _ := d.SpellIdx(spell)
		h.SpellLevels[idx] = lvl
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
	SkillAdd                           map[string]int // spell id -> levels the equipment adds to that one spell
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

	a := h.d.Class(h.Class)
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
		for id, n := range it.SkillAdd {
			if s.SkillAdd == nil {
				s.SkillAdd = map[string]int{}
			}
			s.SkillAdd[id] += n
		}
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
		if a.Flags&defs.ClassTrainedFists != 0 {
			s.MinDam, s.MaxDam = max(s.MinDam, h.Level/2), max(s.MaxDam, h.Level)
		}
	}
	if a.Flags&defs.ClassParry != 0 { // CalcPlrBlockFlag: the Monk parries with a staff, a single one-hander or bare hands
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
	switch { // CalcPlrDamage's damage bonus, which every Hellfire class weighs differently
	case a.Flags&defs.ClassStrDexDamage != 0:
		s.DamageMod = strDex / 200
	case a.Flags&defs.ClassStaffAndFistDamage != 0:
		if s.Holds(ITypeStaff) || bareHanded {
			s.DamageMod = strDex / 150
		} else {
			s.DamageMod = strDex / 300
		}
	case a.Flags&defs.ClassBladeAndBowDamage != 0:
		switch {
		case s.Holds(ITypeSword):
			s.DamageMod = strDex / 150
		case s.Holds(ITypeBow):
			s.DamageMod = strDex / 250
		default:
			s.DamageMod = h.Level * s.Str / 100
		}
	case a.Flags&defs.ClassHeavyWeaponDamage != 0:
		switch {
		case s.Holds(ITypeAxe) || s.Holds(ITypeMace):
			s.DamageMod = h.Level * s.Str / 75
		case s.Holds(ITypeBow):
			s.DamageMod = h.Level * s.Str / 300
		default:
			s.DamageMod = h.Level * s.Str / 100
		}
	default:
		s.DamageMod = h.Level * s.Str / 100
	}
	if hasShieldOnly && a.Flags&defs.ClassHalfShield != 0 { // a Barbarian hides behind only half a shield
		s.AC -= shieldAC / 2
	} else if !hasShieldOnly && a.Flags&defs.ClassVitalityDamage != 0 && !s.Holds(ITypeStaff) && !s.Holds(ITypeBow) {
		s.DamageMod += h.Level * s.Vit / 100
	}
	if a.Flags&defs.ClassIronSkin != 0 {
		s.BonusAC += h.Level / 4
	}
	if a.Flags&defs.ClassNaturalResistance != 0 { // CalcPlrResistances
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
	} else if a.Flags&defs.ClassShieldSlowsBareHand != 0 && hasShieldOnly && s.WeaponType == ITypeMisc {
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
	hper := s.EnAc + s.Dex>>1 - monAC + h.Level + h.d.Class(h.Class).MeleeToHit
	return clamp(hper+s.BonusToHit, 5, 95)
}

// MeleeDamage ports PlrHitMonst's damage roll (whole points). monClass is MC_UNDEAD/MC_DEMON/MC_ANIMAL.
func (h *Hero) MeleeDamage(rng *rand.Rand, s Sheet, monClass string) int {
	d := s.MinDam + rng.Intn(max(s.MaxDam-s.MinDam+1, 1))
	crit := h.d.Class(h.Class).Flags&defs.ClassCriticalStrike != 0
	return h.meleeDamageOf(s, d, crit && rng.Intn(100) < h.Level, monClass)
}

// MeleeDamageRange is MeleeDamage's lowest roll without and highest roll with the warrior's double damage.
func (h *Hero) MeleeDamageRange(s Sheet, monClass string) (lo, hi int) {
	crit := h.d.Class(h.Class).Flags&defs.ClassCriticalStrike != 0 && h.Level > 0
	return h.meleeDamageOf(s, s.MinDam, false, monClass), h.meleeDamageOf(s, s.MaxDam, crit, monClass)
}

func (h *Hero) meleeDamageOf(s Sheet, d int, double bool, monClass string) int {
	dam := s.BonusDamMod + s.DamageMod + d*s.BonusDam/100 + d
	if double {
		dam *= 2
	}
	switch pct := h.d.WeaponVsMonsterClass(s, monClass); {
	case pct == -50: // 1.09's own arithmetic, kept exact
		dam -= dam >> 1
	case pct == 50:
		dam += dam >> 1
	case pct != 0:
		dam += dam * pct / 100
	}
	if s.Flags&ISpl3xDamVsDemons != 0 && monClass == "MC_DEMON" {
		dam *= 3
	}
	return dam
}

// WeaponBonus is one of the mod's weapon rules (hero.rec's weapon_bonus): melee with a weapon of type Weapon
// does Percent more (or less) damage to monsters of class Versus. D1's are PlrHitMonst's phanditype rule, swords
// cut flesh and maces crush bone; a mod without the section has none. Filled by Load.
type WeaponBonus struct {
	Weapon  int
	Versus  string
	Percent int
}

// RuledWeapon is the weapon type the bonuses judge the hero by: the first type the rules name that the hero
// holds, so the file's order settles two weapons (D1: a sword in either hand wins over a mace). -1 for none.
func (d *Data) RuledWeapon(s Sheet) int {
	for _, b := range d.WeaponBonuses {
		if s.Holds(b.Weapon) {
			return b.Weapon
		}
	}
	return -1
}

// WeaponVsMonsterClass is what the weapon in hand does to melee damage against a monster class, as a percentage.
func (d *Data) WeaponVsMonsterClass(s Sheet, monClass string) int {
	return d.WeaponBonusOf(d.RuledWeapon(s), monClass)
}

// WeaponBonusOf is the rules' percentage for weapon type itype against monClass, 0 when none applies.
func (d *Data) WeaponBonusOf(itype int, monClass string) int {
	for _, b := range d.WeaponBonuses {
		if b.Weapon == itype && b.Versus == monClass {
			return b.Percent
		}
	}
	return 0
}

// WeaponTypeName is the name hero.rec gives weapon type itype.
func WeaponTypeName(itype int) string {
	for name, t := range itypeByName {
		if t == itype && name != "default" {
			return name
		}
	}
	return ""
}

// CriticalStrikeChance is the percentage of melee swings that land twice (MeleeDamage's crit roll): the
// character's level, for the classes that have it, and 0 for the rest.
func (h *Hero) CriticalStrikeChance() int {
	if !h.d.HasClassFlag(h.Class, defs.ClassCriticalStrike) {
		return 0
	}
	return min(h.Level, 100)
}

// FullMissileDamageBonus reports a class that adds its whole Str/Dex damage bonus to missiles instead of half of
// it (arrowDamageOf).
func (d *Data) FullMissileDamageBonus(class string) bool {
	return d.Class(class).Flags&defs.ClassFullMissileBonus != 0
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
	if s.Flags&ISplStealLife5 != 0 { // the 5% overrides the 3%, as with mana
		hp += 5 * d / 100
	} else if s.Flags&ISplStealLife3 != 0 {
		hp += 3 * d / 100
	}
	return
}

// ArrowHitChance ports MonsterMHit for physical missiles; dist in cells.
func (h *Hero) ArrowHitChance(s Sheet, monAC, dist int) int {
	hper := h.Level + h.d.Class(h.Class).RangedToHit - monAC + s.EnAc + s.BonusToHit + s.Dex - dist*dist>>1
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
	if h.d.FullMissileDamageBonus(h.Class) {
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
		return max(Rnd(rng, maxd-mind+1)+mind+s.GetHit, 64)
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
		dam, getHit = Rnd(rng, maxd-mind+1)+mind, s.GetHit
	} else {
		dam, getHit = Rnd(rng, (maxd-mind+1)<<6)+mind<<6, s.GetHit<<6
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
	dam := Rnd(rng, (maxDam-minDam+1)<<6) + minDam<<6 + s.GetHit<<6
	return max(dam, 64)
}

// AddExperience ports AddPlrExperience (single player); returns levels gained.
func (h *Hero) AddExperience(monLevel, exp int) int {
	e := int(float64(exp) * (float64(monLevel-h.Level)/10 + 1))
	h.Exp = min(h.Exp+max(e, 0), 2000000000) // MAXEXP
	gained := 0
	if h.Exp >= h.d.ExpLvls[49] { // level 50 outright, without NextPlrLevel's points, life and mana
		gained, h.Level = 50-h.Level, 50
		return gained
	}
	l := 0
	for h.Exp >= h.d.ExpLvls[l] {
		l++
	}
	for ; h.Level < l; gained++ {
		h.nextLevel()
	}
	return gained
}

func (h *Hero) statDiff() int {
	m := h.d.Class(h.Class).MaxStats()
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
	a := h.d.Class(h.Class)
	h.SkillPts += a.SkillPoints
	hp, mana := a.LvlLife+1, a.LvlMana+1 // single player: one extra 1/64 of each per level
	h.MaxHPBase += hp
	h.HPBase = h.MaxHPBase
	h.MaxManaBase += mana
	h.ManaBase = h.MaxManaBase // ponytail: D1 skips this refill with ISPL_NOMANA equipped; caller zeroes mana then.
}

// AddBaseStat ports ModifyPlr{Str,Mag,Dex,Vit}: raises a base attribute (capped) and returns HP/mana gained in 1/64.
func (h *Hero) AddBaseStat(attr, n int) (hp, mana int) {
	base := [4]*int{&h.BaseStr, &h.BaseMag, &h.BaseDex, &h.BaseVit}[attr]
	n = max(min(n, h.d.Class(h.Class).MaxStats()[attr]-*base), -*base) // ModifyPlr* caps at MaxStats, CheckStats floors at 0
	if n == 0 {
		return 0, 0
	}
	*base += n
	switch attr {
	case AttrMag:
		mana = n * h.d.Class(h.Class).ChrMana
		h.MaxManaBase += mana
		h.ManaBase += mana
	case AttrVit:
		hp = n * h.d.Class(h.Class).ChrLife
		h.MaxHPBase += hp
		h.HPBase += hp
	}
	return
}

// SpendStatPoint spends one level-up point on an attribute; false if none left or capped.
func (h *Hero) SpendStatPoint(attr int) bool {
	base := [4]int{h.BaseStr, h.BaseMag, h.BaseDex, h.BaseVit}[attr]
	if h.StatPts <= 0 || base >= h.d.Class(h.Class).MaxStats()[attr] {
		return false
	}
	h.AddBaseStat(attr, 1)
	h.StatPts--
	return true
}

// Learnable reports whether the hero's class may put skill points into the spell: classes.rec's start_spells lists
// it (at level 0 when the hero doesn't know it yet).
func (h *Hero) Learnable(spell int) bool {
	_, ok := h.d.Class(h.Class).StartSpells[h.d.Spells[spell].ID]
	return ok
}

// SpendSkillPoint puts one level-up point into a learnable spell, up to the class's max_skill_level; false if it can't.
func (h *Hero) SpendSkillPoint(spell int) bool {
	if h.SkillPts <= 0 || !h.Learnable(spell) || h.SpellLevels[spell] >= h.d.Class(h.Class).MaxSkillLevel {
		return false
	}
	h.SpellLevels[spell]++
	h.SkillPts--
	return true
}

// potionAmount is RestorePartialLife/RestorePartialMana in 1/64 for roll in [0, maxV>>8); factor is the class'
// 1/64 multiplier on the roll.
func potionAmount(maxV, roll, factor int) int {
	return (maxV>>8>>1 + roll) << 6 * factor >> 6
}

// PotionHeal ports UseItem's IMISC_HEAL amount in 1/64; maxHP in 1/64.
func (h *Hero) PotionHeal(rng *rand.Rand, maxHP int) int {
	return potionAmount(maxHP, rng.Intn(max(maxHP>>8, 1)), h.d.Class(h.Class).PotionLife)
}

// PotionMana ports UseItem's IMISC_MANA amount in 1/64; maxMana in 1/64.
func (h *Hero) PotionMana(rng *rand.Rand, maxMana int) int {
	return potionAmount(maxMana, rng.Intn(max(maxMana>>8, 1)), h.d.Class(h.Class).PotionMana)
}

// PotionHealRange and PotionManaRange are the lowest and highest PotionHeal/PotionMana rolls, in 1/64.
func (h *Hero) PotionHealRange(maxHP int) (int, int) {
	f := h.d.Class(h.Class).PotionLife
	return potionAmount(maxHP, 0, f), potionAmount(maxHP, max(maxHP>>8, 1)-1, f)
}

func (h *Hero) PotionManaRange(maxMana int) (int, int) {
	f := h.d.Class(h.Class).PotionMana
	return potionAmount(maxMana, 0, f), potionAmount(maxMana, max(maxMana>>8, 1)-1, f)
}

// RefWalkTicks is D1's walk step in ticks, one 10-AUT turn: the D1 clock (decision 13).
const RefWalkTicks = 8

// SetClassFlagForTest gives class a class flag, as a classes.rec line would; for tests that move a rule between
// classes.
func (d *Data) SetClassFlagForTest(class string, flag uint32) { d.Class(class).Flags |= flag }
