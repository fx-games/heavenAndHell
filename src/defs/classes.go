package defs

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"rpg/src/recfile"
)

// ClassIcons resolves a class' two sprites from the active mod's actors.rec by the name the mod's classes.rec
// gave the class: "archer" wants class.archer_male and class.archer_female. The engine keeps no roster of class
// names - which classes exist is the mod's business - so a name the mod's icon table does not carry resolves to
// 0 here rather than failing LoadIcons, and is reported when the class is read.
func ClassIcons(internalName string) (male, female int32, ok bool) {
	male, maleOK := IconOK(IconID("class", internalName+"_male"))
	female, femaleOK := IconOK(IconID("class", internalName+"_female"))
	return male, female, maleOK && femaleOK
}

// Class flags: whatever a class does differently is one of these, any class's to take.
const (
	ClassCriticalStrike    = 1 << iota // clvl% of melee swings hit twice
	ClassTrapSense                     // trapped doors and chests read as trapped
	ClassIronSkin                      // +clvl/4 armor class
	ClassNaturalResistance             // +clvl to every resistance
	ClassDualWield                     // may hold a weapon in the off hand (not modelled, see docs/D1_PLAN.md)
	// What used to be `switch h.Class`: each is the rule one of Hellfire's classes had, now any class's to take.
	ClassTrainedFists        // bare fists hit for clvl/2..clvl (the Monk's)
	ClassParry               // blocks with a staff, one one-hander or bare hands; the staff blocks fast (the Monk's)
	ClassStrDexDamage        // melee bonus clvl*(Str+Dex)/200 (the Rogue's)
	ClassStaffAndFistDamage  // clvl*(Str+Dex)/150 with a staff or bare hands, /300 otherwise (the Monk's)
	ClassBladeAndBowDamage   // clvl*(Str+Dex)/150 with a sword, /250 with a bow, else clvl*Str/100 (the Bard's)
	ClassHeavyWeaponDamage   // clvl*Str/75 with an axe or mace, /300 with a bow, else /100 (the Barbarian's)
	ClassVitalityDamage      // +clvl*Vit/100 melee bonus, unless shield-only or with a staff or bow (the Barbarian's)
	ClassHalfShield          // a shield without a weapon is worth only half its AC (the Barbarian's)
	ClassFullMissileBonus    // missiles get the whole damage bonus, not half (the Rogue's and the Bard's)
	ClassShieldSlowsBareHand // bare hands behind a shield attack in 16 frames (the Sorcerer's)
	ClassFlinchResist        // hit recovery needs clvl/4 more damage to start (the Barbarian's)
)

// ClassDamageFlags are the melee damage bonus formulas; a class takes at most one (none: clvl*Str/100).
const ClassDamageFlags = ClassStrDexDamage | ClassStaffAndFistDamage | ClassBladeAndBowDamage | ClassHeavyWeaponDamage

var classFlagByName = map[string]uint32{"critical_strike": ClassCriticalStrike, "trap_sense": ClassTrapSense,
	"iron_skin": ClassIronSkin, "natural_resistance": ClassNaturalResistance, "dual_wield": ClassDualWield,
	"trained_fists": ClassTrainedFists, "parry": ClassParry, "str_dex_damage": ClassStrDexDamage,
	"staff_and_fist_damage": ClassStaffAndFistDamage, "blade_and_bow_damage": ClassBladeAndBowDamage,
	"heavy_weapon_damage": ClassHeavyWeaponDamage, "vitality_damage": ClassVitalityDamage,
	"half_shield": ClassHalfShield, "full_missile_bonus": ClassFullMissileBonus,
	"shield_slows_bare_hands": ClassShieldSlowsBareHand, "flinch_resist": ClassFlinchResist}

// ClassAttrNames name oily's attributes, in the hero's attribute order (str, mag, dex, vit).
var ClassAttrNames = []string{"str", "mag", "dex", "vit"}

// ClassRules are a class's numbers and rules: the classes.rec fields past the menu's. The 1/64 columns are
// devilutionX's parseFixed6 multipliers, written in the file as decimals.
type ClassRules struct {
	Str, Mag, Dex, Vit                  int
	MaxStr, MaxMag, MaxDex, MaxVit      int
	ToBlk                               int
	AdjLife, AdjMana                    int // flat part of the starting pool, 1/64
	LvlLife, LvlMana                    int // gained per character level, 1/64
	ChrLife, ChrMana                    int // gained per point of Vitality / Magic, 1/64
	ItmLife, ItmMana                    int // multiplier on Vitality / Magic from items, 1/64
	PotionLife, PotionMana              int // multiplier on a potion's roll, 1/64
	SpellLife                           int // multiplier on the Healing spell's roll, 1/64
	ManaCost                            int // multiplier on a spell's mana cost, 1/64
	MagicToHit, MeleeToHit, RangedToHit int
	AttackFrames                        map[int]int // attack animation length by item type (ItemTypeNames), 0 = bare-handed and the default
	SpellFrames, HitFrames              int
	Flags                               uint32
	StartSpells                         map[string]int // spell levels a new hero knows, by spell id; 0: learnable, not known yet
	SkillPoints                         int            // skill points gained per character level
	MaxSkillLevel                       int            // how far skill points raise a spell (books and gear go past it)
	Key                                 string         // what spells.rec's effects call the class ("warrior")
	SpellToHit                          int            // added to magic missiles' to-hit
	HealBoost                           int            // Healing and potions: 2 doubles the roll, 1 adds half
	Skill                               string         // the class skill; "" for none
	Voice, FemaleVoice                  string         // PS_<voice>nn sound set
	DeathSound                          string         // said on death instead of PS_<voice>71
	TextPrefix                          string         // quest books' class version, TEXT_<prefix>BONER
	Oily                                [][2]int       // the Oily shrine's gains: attribute (ClassAttrNames), points
	SlainHero                           string         // the Slain Hero's reward
}

// HasFlag reports a class flag, e.g. ClassTrapSense.
func (r *ClassRules) HasFlag(flag uint32) bool { return r.Flags&flag != 0 }

// MaxStats are the attribute caps, in ClassAttrNames order.
func (r *ClassRules) MaxStats() [4]int { return [4]int{r.MaxStr, r.MaxMag, r.MaxDex, r.MaxVit} }

type LoadoutDefinition struct {
	ClassRules
	Items        []string
	Icon         int32
	Gold         int
	InternalName string
	Name         string
	FemaleIcon   int32 // the other sprite; Icon carries whichever gender was picked
}

// IsFemale reports which of the class' two sprites this loadout carries. Saving and the hero's voice ask it.
func (d LoadoutDefinition) IsFemale() bool { return d.Icon == d.FemaleIcon }

// ReadClasses reads the mod's classes.rec, one record per class, in the order the class menu lists them. Nothing
// but the menu may rely on that order: a class is found by its internal_name. A mod without the file has no
// classes (fs.ErrNotExist).
func ReadClasses(fsys fs.FS) ([]LoadoutDefinition, error) {
	f, err := fsys.Open(path.Join("assets", "data", "classes.rec"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var classes []LoadoutDefinition
	seen := map[string]bool{}
	for _, r := range recfile.Read(f) {
		c, err := newClass(r)
		if err != nil {
			return nil, fmt.Errorf("classes.rec: %s: %w", c.InternalName, err)
		}
		if c.InternalName == "" || seen[c.InternalName] {
			return nil, fmt.Errorf("classes.rec: internal_name %q is missing or used twice", c.InternalName)
		}
		seen[c.InternalName] = true
		classes = append(classes, c)
	}
	if len(classes) == 0 {
		return nil, errors.New("classes.rec: no classes")
	}
	return classes, nil
}

func newClass(r recfile.Record) (LoadoutDefinition, error) {
	m := r.ToMap("")
	fp := func(key string) int { return int(m.GetFloatOrDefault(key, 0) * 64) }
	num := m.GetIntOrDefault
	c := LoadoutDefinition{InternalName: m.GetStringOrDefault("internal_name", ""),
		Name: m.GetStringOrDefault("name", ""), Gold: num("gold", 0)}
	c.ClassRules = ClassRules{
		Str: num("str", 0), Mag: num("mag", 0), Dex: num("dex", 0), Vit: num("vit", 0),
		MaxStr: num("max_str", 0), MaxMag: num("max_mag", 0), MaxDex: num("max_dex", 0), MaxVit: num("max_vit", 0),
		ToBlk: num("to_blk", 0), SkillPoints: num("skill_points", 0), MaxSkillLevel: num("max_skill_level", 0),
		AdjLife: fp("adj_life"), AdjMana: fp("adj_mana"), LvlLife: fp("lvl_life"), LvlMana: fp("lvl_mana"),
		ChrLife: fp("chr_life"), ChrMana: fp("chr_mana"), ItmLife: fp("itm_life"), ItmMana: fp("itm_mana"),
		PotionLife: fp("potion_life"), PotionMana: fp("potion_mana"), SpellLife: fp("spell_life"),
		ManaCost: fp("mana_cost"), MagicToHit: num("magic_to_hit", 0), MeleeToHit: num("melee_to_hit", 0),
		RangedToHit: num("ranged_to_hit", 0), SpellToHit: num("spell_to_hit", 0), HealBoost: num("heal_boost", 0),
		SpellFrames: num("spell_frames", 0), HitFrames: num("hit_frames", 0),
		AttackFrames: map[int]int{}, StartSpells: map[string]int{},
		Key:   m.GetStringOrDefault("key", strings.ToLower(c.Name)),
		Skill: m.GetStringOrDefault("skill", ""), TextPrefix: m.GetStringOrDefault("text_prefix", ""),
		Voice: m.GetStringOrDefault("voice", "WARR"), DeathSound: m.GetStringOrDefault("death_sound", ""),
		SlainHero: m.GetStringOrDefault("slain_hero", ""),
	}
	c.FemaleVoice = m.GetStringOrDefault("female_voice", c.Voice)
	for _, f := range r {
		switch f.Name {
		case "item":
			c.Items = append(c.Items, f.Value)
		case "oily":
			attr, pts, _ := strings.Cut(strings.TrimSpace(f.Value), " ")
			at := slices.Index(ClassAttrNames, attr)
			n, err := strconv.Atoi(strings.TrimSpace(pts))
			if at < 0 || err != nil {
				return c, fmt.Errorf("oily %q: want <str|mag|dex|vit> <points>", f.Value)
			}
			c.Oily = append(c.Oily, [2]int{at, n})
		}
	}
	for _, f := range strings.Fields(m.GetStringOrDefault("attack_frames", "")) {
		name, frames, _ := strings.Cut(f, "=")
		itype := slices.Index(ItemTypeNames, name)
		if name == "default" {
			itype = 0
		}
		n, err := strconv.Atoi(frames)
		if itype < 0 || err != nil {
			return c, fmt.Errorf("bad attack_frames %q", f)
		}
		c.AttackFrames[itype] = n
	}
	for _, f := range strings.Fields(m.GetStringOrDefault("start_spells", "")) {
		name, lvl, _ := strings.Cut(f, "=")
		n, err := strconv.Atoi(lvl)
		if err != nil {
			return c, fmt.Errorf("bad start_spells %q", f)
		}
		c.StartSpells[name] = n
	}
	for _, f := range strings.Fields(m.GetStringOrDefault("flags", "")) {
		bit, ok := classFlagByName[f]
		if !ok {
			return c, fmt.Errorf("unknown class flag %q", f)
		}
		c.Flags |= bit
	}
	if n := c.Flags & ClassDamageFlags; n&(n-1) != 0 {
		return c, errors.New("more than one damage formula flag")
	}
	c.Icon, c.FemaleIcon, _ = ClassIcons(c.InternalName)
	return c, nil
}
