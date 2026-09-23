package rules

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"math/rand"
	"rpg/src/recfile"
	"strings"
)

// Spell ids (SPL_) of the 26 single player spells (decision 6), plus Hellfire's. Rage is Hellfire's use of D1's
// unused SPL_BLODBOIL slot; the rest are Hellfire's table past D1's 37, in its order.
const (
	SplFirebolt    = 1
	SplHeal        = 2
	SplLightning   = 3
	SplFlash       = 4
	SplIdentify    = 5
	SplFireWall    = 6
	SplTown        = 7
	SplStone       = 8
	SplInfra       = 9
	SplPhasing     = 10
	SplManaShield  = 11
	SplFireball    = 12
	SplGuardian    = 13
	SplChain       = 14
	SplWave        = 15
	SplNova        = 18
	SplInferno     = 20
	SplGolem       = 21
	SplRage        = 22 // the Barbarian's skill (SPL_BLODBOIL)
	SplTeleport    = 23
	SplApocalypse  = 24
	SplRepair      = 26 // the class skills (_pAblSpells)
	SplRecharge    = 27
	SplDisarm      = 28
	SplElemental   = 29
	SplChargedBolt = 30
	SplHolyBolt    = 31
	SplTelekinesis = 33
	SplBloodStar   = 35
	SplBoneSpirit  = 36
	// Hellfire's
	SplMana        = 37 // staff only
	SplMagi        = 38 // staff only
	SplJester      = 39 // staff only
	SplLightWall   = 40
	SplImmolation  = 41
	SplWarp        = 42
	SplReflect     = 43
	SplBerserk     = 44
	SplRingOfFire  = 45
	SplSearch      = 46 // also the Monk's skill
	SplRuneFire    = 47 // the runes' spells; the rune items cast them
	SplRuneLight   = 48
	SplRuneNova    = 49
	SplRuneImmolat = 50
	SplRuneStone   = 51
	NumSpells      = 52
	// ClassicSpellCount is D1 1.09's MAX_SPELLS; Hellfire's is NumSpells.
	ClassicSpellCount = 37
)

// Missile resistance kinds (missiledata mResist).
const (
	ResistNone = iota
	ResistKindFire
	ResistKindLightning
	ResistKindMagic
)

// spellIDByName binds D1's SPL_ ids to this package's constants, so spells.rec can be keyed by the name D1 uses
// instead of by the raw slot number.
var spellIDByName = map[string]int{
	"SPL_FIREBOLT":    SplFirebolt,
	"SPL_HEAL":        SplHeal,
	"SPL_LIGHTNING":   SplLightning,
	"SPL_FLASH":       SplFlash,
	"SPL_IDENTIFY":    SplIdentify,
	"SPL_FIREWALL":    SplFireWall,
	"SPL_TOWN":        SplTown,
	"SPL_STONE":       SplStone,
	"SPL_INFRA":       SplInfra,
	"SPL_RNDTELEPORT": SplPhasing,
	"SPL_MANASHIELD":  SplManaShield,
	"SPL_FIREBALL":    SplFireball,
	"SPL_GUARDIAN":    SplGuardian,
	"SPL_CHAIN":       SplChain,
	"SPL_WAVE":        SplWave,
	"SPL_NOVA":        SplNova,
	"SPL_INFERNO":     SplInferno,
	"SPL_GOLEM":       SplGolem,
	"SPL_BLODBOIL":    SplRage,
	"SPL_TELEPORT":    SplTeleport,
	"SPL_APOCA":       SplApocalypse,
	"SPL_REPAIR":      SplRepair,
	"SPL_RECHARGE":    SplRecharge,
	"SPL_DISARM":      SplDisarm,
	"SPL_ELEMENT":     SplElemental,
	"SPL_CBOLT":       SplChargedBolt,
	"SPL_HBOLT":       SplHolyBolt,
	"SPL_RESURRECT":   SplTelekinesis,
	"SPL_BLODSTAR":    SplBloodStar,
	"SPL_BONESPIRIT":  SplBoneSpirit,
	"SPL_MANA":        SplMana,
	"SPL_MAGI":        SplMagi,
	"SPL_JESTER":      SplJester,
	"SPL_LIGHTWALL":   SplLightWall,
	"SPL_IMMOLAT":     SplImmolation,
	"SPL_WARP":        SplWarp,
	"SPL_REFLECT":     SplReflect,
	"SPL_BERSERK":     SplBerserk,
	"SPL_RING":        SplRingOfFire,
	"SPL_SEARCH":      SplSearch,
	"SPL_RUNEFIRE":    SplRuneFire,
	"SPL_RUNELIGHT":   SplRuneLight,
	"SPL_RUNENOVA":    SplRuneNova,
	"SPL_RUNEIMMOLAT": SplRuneImmolat,
	"SPL_RUNESTONE":   SplRuneStone,
}

var resistKindByName = map[string]int{"none": ResistNone, "fire": ResistKindFire,
	"lightning": ResistKindLightning, "magic": ResistKindMagic}

//go:embed skills.rec
var engineSkills string

// loadSpellTables reads the engine's own skills (skills.rec, embedded) and then the mod's spells.rec. A record
// keyed by a D1 SPL_ name fills that slot; any other id is a named skill, a row after D1's NumSpells. A mod record
// with a named skill's id replaces the engine's whole record, so a mod needs no record to use a skill.
func loadSpellTables(fsys fs.FS, d *Data) error {
	d.Spells = d.Spells[:NumSpells] // a reload starts from D1's slots again
	recs, err := readRecMulti(fsys, "spells.rec")
	// A mod that does not have this Diablo feature ships no file for it, and an absent feature is not an
	// error: what the table would have filled stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		recs, err = map[string][]recfile.Record{}, nil
	}
	if err != nil {
		return err
	}
	records := append(recfile.ReadMulti(strings.NewReader(engineSkills))["default"], recs["default"]...)
	for _, r := range records {
		m := r.ToMap("")
		name := m.GetStringOrDefault("id", "")
		spell, ok := spellIDByName[name]
		if strings.HasPrefix(name, "SPL_") && (!ok || spell >= len(d.Spells)) {
			return fmt.Errorf("spells.rec: unknown spell %q", name)
		}
		if name == "" {
			return fmt.Errorf("spells.rec: a record without an id")
		}
		sp := &SpellData{}
		if ok {
			sp = &d.Spells[spell]
		} else {
			// a named skill carries the numbers xl_spell holds for D1's
			*sp = SpellData{ID: name, Name: m.GetStringOrDefault("name", name),
				Description: m.GetStringOrDefault("description", ""),
				Mana:        m.GetIntOrDefault("mana", 0), ManaAdj: m.GetIntOrDefault("mana_adj", 0),
				MinMana: m.GetIntOrDefault("min_mana", 0), MinInt: m.GetIntOrDefault("min_int", 0),
				BookLvl: -1, StaffLvl: -1, Town: m.GetBoolOrFalse("town"),
				Targeting: m.GetStringOrDefault("targeting", ""), Sustained: m.GetBoolOrFalse("sustained"),
				Icon: int32(m.GetIntOrDefault("icon", 0))}
		}
		if v := m.GetStringOrDefault("resist", ""); v != "" {
			kind, known := resistKindByName[v]
			if !known {
				return fmt.Errorf("spells.rec: %s: unknown resist %q", name, v)
			}
			sp.Resist = kind
		}
		sp.Targeted = m.GetBoolOrFalse("targeted")
		sp.SelfTarget = m.GetBoolOrFalse("allow_self_target")
		sp.Sfx = [3]string{m.GetStringOrDefault("cast_sfx", ""),
			m.GetStringOrDefault("launch_sfx", ""), m.GetStringOrDefault("impact_sfx", "")}
		// a spell may declare `effect:` more than once; they all fire over the same hit zone, in order
		for _, f := range r {
			if f.Name == "effect" && f.Value != "" {
				sp.Effect = append(sp.Effect, f.Value)
			}
		}
		if len(sp.Effect) > 0 {
			sp.HitZone = m.GetStringOrDefault("hit_zone", "target_only")
		}
		sp.HealthCost = m.GetIntOrDefault("health_cost", 0)
		if ok {
			continue
		}
		if at, dup := d.SpellIdx(name); dup {
			d.Spells[at] = *sp
		} else {
			d.Spells = append(d.Spells, *sp)
		}
	}
	for i, r := range recs["slot"] {
		m := r.ToMap("")
		if i >= NumSpells {
			return fmt.Errorf("spells.rec: %d spell slots, at most %d", len(recs["slot"]), NumSpells)
		}
		if i < len(d.Spells) {
			d.Spells[i].Name = m.GetStringOrDefault("name", "")
			d.Spells[i].Description = m.GetStringOrDefault("description", "")
		}
	}
	d.RuneTraps = map[string]RuneTrap{}
	for _, r := range recs["rune"] {
		m := r.ToMap("")
		name := m.GetStringOrDefault("spell", "")
		spell, ok := spellIDByName[name]
		if !ok {
			return fmt.Errorf("spells.rec: rune: unknown spell %q", name)
		}
		d.RuneTraps[m.GetStringOrDefault("id", "")] = RuneTrap{Spell: spell, Effect: m.GetStringOrDefault("effect", "")}
	}
	return nil
}

// SpellIdx is the row of the spell or skill with that id (a D1 SPL_ name or a named skill's id).
func (d *Data) SpellIdx(id string) (int, bool) {
	if spell, ok := spellIDByName[id]; ok {
		return spell, true
	}
	for i := range d.Spells {
		if id != "" && d.Spells[i].ID == id {
			return i, true
		}
	}
	return 0, false
}

// RuneTrap is one of Hellfire's placeable traps (the runes), by the IMISC_ id of the item that places it: the
// SPL_ it goes off as, and what the player is told happens.
type RuneTrap struct {
	Spell  int
	Effect string
}

// SpellLevel ports GetSpellLevel.
// A skill's own bonus from equipment (SkillAdd) comes on top, but only once the hero knows the skill.
func (h *Hero) SpellLevel(s Sheet, spell int) int {
	if spell >= len(h.SpellLevels) {
		return 0
	}
	own := 0
	if h.SpellLevels[spell] > 0 {
		own = s.SkillAdd[h.d.Spells[spell].ID]
	}
	return max(h.SpellLevels[spell]+s.SplLvlAdd+own, 0)
}

// ManaCost ports GetManaAmount (1/64).
func (d *Data) ManaCost(h *Hero, s Sheet, spell int) int {
	sd := d.Spells[spell]
	sl := max(h.SpellLevel(s, spell)-1, 0)
	adj := sl * sd.ManaAdj
	if spell == SplFirebolt {
		adj >>= 1
	}
	ma := (sd.Mana - adj) << 6
	if sd.Mana == 255 { // Hellfire's staff-only spells: the low byte of the base max mana, as D1 casts it
		ma = (int(byte(h.MaxManaBase)) - adj) << 6
	}
	if spell == SplHeal {
		ma = (sd.Mana + 2*h.Level - adj) << 6
	}
	ma = ma * d.Class(h.Class).ManaCost >> 6
	if sd.MinMana > ma>>6 {
		ma = sd.MinMana << 6
	}
	return ma
}

// SpellHitChance ports MonsterMHit's to-hit for magic missiles. Its distance term is always 0 for them: only
// MI_Arrow and MI_LArrow ever advance _midist.
func (h *Hero) SpellHitChance(s Sheet, mlvl int) int {
	hper := s.Mag + 50 - mlvl<<1
	hper += h.d.Class(h.Class).SpellToHit
	return clamp(hper, 5, 95)
}

func Rnd(rng *rand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	return rng.Intn(n)
}

// grow adds 1/8 per spell level, like the damage loops in AddFireball and friends.
func grow(v, slvl int) int {
	for ; slvl > 0; slvl-- {
		v += v >> 3
	}
	return v
}

// Damage rolls in whole points unless the name ends in 64 (per D1 tick, 1/64).

func FireboltDamage(rng *rand.Rand, mag, slvl int) int { return mag>>3 + Rnd(rng, 10) + slvl + 1 }
func BloodStarDamage(mag, slvl int) int                { return mag>>1 + 3*slvl - mag>>3 }
func LightningDamage(rng *rand.Rand, clvl int) int     { return Rnd(rng, 2) + Rnd(rng, clvl) + 2 }
func HolyBoltDamage(rng *rand.Rand, clvl int) int      { return Rnd(rng, 10) + clvl + 9 }
func ChargedBoltDamage(rng *rand.Rand, mag int) int    { return Rnd(rng, mag>>2) + 1 }
func ChargedBolts(slvl int) int                        { return slvl>>1 + 3 }
func WaveDamage(rng *rand.Rand, clvl int) int          { return Rnd(rng, 10) + clvl + 1 }

func FireballDamage(rng *rand.Rand, clvl, slvl int) int {
	return grow(2*(Rnd(rng, 10)+Rnd(rng, 10)+clvl+2), slvl)
}

func ElementalDamage(rng *rand.Rand, clvl, slvl int) int { return FireballDamage(rng, clvl, slvl) >> 1 }

func NovaDamage(rng *rand.Rand, clvl, slvl int) int {
	d := 0
	for i := 0; i < 5; i++ {
		d += Rnd(rng, 6)
	}
	return grow((d+clvl+5)>>1, slvl)
}

func ApocalypseDamage(rng *rand.Rand, clvl int) int {
	d := 0
	for i := 0; i < clvl; i++ {
		d += Rnd(rng, 6) + 1
	}
	return d
}

// FlashDamage64 returns the per-tick damage of the flash below and around the caster, and of the flash above.
func FlashDamage64(rng *rand.Rand, clvl, slvl int) (low, high int) {
	for i := 0; i <= clvl; i++ {
		low += Rnd(rng, 20) + 1
		high += Rnd(rng, 2) + 1
	}
	low, high = grow(low, slvl), grow(high, slvl)
	return low + low>>1, high + high>>1
}

const FlashTicks = 19

// FlashRadius is how far the burst of light reaches: every cell within one square of the caster.
const FlashRadius = 1

// BoomTries: an Apocalypse blast (MI_Boom) rolls to hit every tick of its 15-frame animation until one lands.
const BoomTries = 15

// ElementalBurstTicks: Elemental's explosion (MFILE_BIGEXP, 15 frames) hits its 3×3 every tick but the last, each
// tick for the missile's damage read as 1/64.
const ElementalBurstTicks = 14

// HellfireFireWallDamage64 is Hellfire's AddFirewall: a misplaced parenthesis turns the roll into plr._pLevel.
func HellfireFireWallDamage64(clvl int) int { return clvl << 4 >> 1 }

// FireWallTurns ports AddFirewall's range (ticks) in turns; Hellfire's walls, which burn the hero too
// (TARGET_BOTH), burn dlvl ticks·16 longer, pass 0 for D1's.
func FireWallTurns(slvl, dlvl int) int { return (10*(slvl+1) + dlvl) * 16 / RefWalkTicks }

// LightningTicks is how long each cell of a Lightning or Chain Lightning bolt burns (AddLightning's range), hitting
// every tick.
func LightningTicks(slvl int) int { return slvl>>1 + 6 }

// ManaShieldCost ports MI_Manashield's reduction of the life lost (diff) before mana pays for it.
func ManaShieldCost(diff, slvl int) int {
	if slvl > 0 { // 1.09; Hellfire divides by 3·min(slvl, 7) instead
		diff -= diff / 3
	}
	return max(diff, 0)
}

func InfernoDamage64(rng *rand.Rand, clvl int) int {
	d := 8 * (Rnd(rng, 2) + Rnd(rng, clvl) + 2)
	return d + d>>1
}

// InfernoTicks: how long the flame n cells (0-2) from the caster burns.
func InfernoTicks(n int) int { return 5*n + 20 }

func GuardianTurns(clvl, slvl int) int {
	r := min(slvl+clvl>>1, 30) << 4
	return max(r, 30) / RefWalkTicks
}

func StoneTurns(slvl int) int { return min(slvl+6, 15) << 4 / RefWalkTicks }

func InfravisionTurns(slvl int) int { return grow(1584, slvl) / RefWalkTicks }

// ResourceAmount64 is the curve AddHeal and missiles_rech_mana share, in 1/64 points: 1d<base>, plus
// 1d<perClvl> for every character level and 1d<perSlvl> for every spell level, then the multiplier the
// spell gives the classes it favours - boost 2 doubles the roll, 1 raises it by half, 0 leaves it alone.
// Which classes those are is the content's business; only the shape lives here.
func ResourceAmount64(rng *rand.Rand, base, perClvl, perSlvl, clvl, slvl, boost int) int {
	l := (Rnd(rng, base) + 1) << 6
	for i := 0; i < clvl; i++ {
		l += (Rnd(rng, perClvl) + 1) << 6
	}
	for i := 0; i < slvl; i++ {
		l += (Rnd(rng, perSlvl) + 1) << 6
	}
	switch boost {
	case 2:
		l *= 2
	case 1:
		l += l >> 1
	}
	return l
}

// HealAmount64 ports AddHeal: the shared curve with Healing's own class multipliers. It stays because the
// item tooltips and HealRange64 quote a healing staff's range without casting anything.
func (d *Data) HealAmount64(rng *rand.Rand, class string, clvl, slvl int) int {
	return ResourceAmount64(rng, 10, 4, 6, clvl, slvl, d.Class(class).HealBoost)
}

// HealRange64 is HealAmount64's lowest and highest roll.
func (d *Data) HealRange64(class string, clvl, slvl int) (int, int) {
	lo, hi := (1+clvl+slvl)<<6, (10+4*clvl+6*slvl)<<6
	switch d.Class(class).HealBoost {
	case 2:
		return lo * 2, hi * 2
	case 1:
		return lo + lo>>1, hi + hi>>1
	}
	return lo, hi
}

// SpellDamageRange is the lowest and highest roll of a direct-damage spell's hit (whole points); ok is false for
// spells without one. Charged Bolt's range is per spark (see ChargedBolts).
func SpellDamageRange(spell, mag, clvl, slvl int) (lo, hi int, ok bool) {
	switch spell {
	case SplFirebolt, SplGuardian: // the Guardian spits firebolts
		return mag>>3 + slvl + 1, mag>>3 + slvl + 10, true
	case SplHolyBolt:
		return clvl + 9, clvl + 18, true
	case SplBloodStar:
		d := BloodStarDamage(mag, slvl)
		return d, d, true
	case SplLightning, SplChain: // a full burn on one cell
		return 2 * LightningTicks(slvl), (max(clvl-1, 0) + 3) * LightningTicks(slvl), true
	case SplChargedBolt:
		return 1, max(mag>>2, 1), true
	case SplWave:
		return clvl + 1, clvl + 10, true
	case SplFireball:
		return grow(2*(clvl+2), slvl), grow(2*(clvl+20), slvl), true
	case SplElemental:
		return grow(2*(clvl+2), slvl) >> 1, grow(2*(clvl+20), slvl) >> 1, true
	case SplNova, SplImmolation:
		return grow((clvl+5)>>1, slvl), grow((clvl+30)>>1, slvl), true
	case SplApocalypse:
		return clvl, 6 * clvl, true
	case SplInferno: // a full burn on the cell next to the caster
		t := 12 * InfernoTicks(0)
		return t * 2 >> 6, t * (max(clvl-1, 0) + 3) >> 6, true
	case SplFlash: // a full flash on the caster's row and the one below
		lo, hi := grow(clvl+1, slvl), grow(20*(clvl+1), slvl)
		return (lo + lo>>1) * FlashTicks >> 6, (hi + hi>>1) * FlashTicks >> 6, true
	case SplFireWall, SplRingOfFire: // per turn
		return 8 * (clvl + 2) * RefWalkTicks >> 6, 8 * (clvl + 20) * RefWalkTicks >> 6, true
	case SplLightWall: // per turn
		return 16 * (clvl + 2) * RefWalkTicks >> 6, 16 * (clvl + 20) * RefWalkTicks >> 6, true
	}
	return 0, 0, false
}

// SpellDamageText is the tooltip line for SpellDamageRange, or "" for spells that deal no direct damage.
func (d *Data) SpellDamageText(spell, mag, clvl, slvl int) string {
	if spell == SplBoneSpirit {
		return "Magic damage: a third of the target's life"
	}
	lo, hi, ok := SpellDamageRange(spell, mag, clvl, slvl)
	kind := [...]string{"", "fire ", "lightning ", "magic "}[d.Spells[spell].Resist]
	if !ok {
		if kind == "" {
			return ""
		}
		return strings.ToUpper(kind[:1]) + kind[1:] + "spell" // no range to show, but monsters still resist it by type
	}
	t := fmt.Sprintf("%d-%d %sdamage", lo, hi, kind)
	if lo == hi {
		t = fmt.Sprintf("%d %sdamage", lo, kind)
	}
	if spell == SplChargedBolt {
		t += fmt.Sprintf(" per spark, %d sparks", ChargedBolts(slvl))
	}
	if spell == SplFireWall || spell == SplRingOfFire || spell == SplLightWall {
		t += " per turn"
	}
	return t
}

// Golem ports SpawnGolum's stats; maxMana in 1/64, HP returned in whole points.
type Golem struct{ HP, AC, Hit, MinDam, MaxDam int }

func GolemStats(slvl, clvl, maxMana int) Golem {
	return Golem{HP: 2 * (320*slvl + maxMana/3) >> 6, AC: 25, Hit: 5*(slvl+8) + 2*clvl, MinDam: 2 * (slvl + 4), MaxDam: 2 * (slvl + 8)}
}

// NovaRays are the offsets MI_Nova sends lightning balls towards (VCrawlTable[k][6..7], mirrored).
var NovaRays = func() [][2]int {
	base := [][2]int{{4, 0}, {4, 0}, {4, 1}, {4, 1}, {4, 1}, {4, 1}, {4, 2}, {4, 2}, {4, 2}, {4, 3}, {4, 4}, {4, 4},
		{4, 4}, {3, 4}, {2, 4}, {2, 4}, {2, 4}, {1, 4}, {1, 4}, {1, 4}, {1, 4}, {0, 4}, {0, 4}}
	var rays [][2]int
	prev := [2]int{}
	for _, r := range base {
		if r == prev {
			continue
		}
		rays = append(rays, [2]int{r[0], r[1]}, [2]int{-r[0], -r[1]}, [2]int{-r[0], r[1]}, [2]int{r[0], -r[1]})
		prev = r
	}
	return rays
}()

// RageTurns and RageDamage port AddRage/ProcessRage: the Barbarian rages for that many turns, drags through the
// same again with the bonus reversed, and pays 6 HP per character level at the end.
func RageTurns(clvl int) int  { return (245 + 2*clvl) / RefWalkTicks }
func RageDamage(clvl int) int { return 6 * clvl }

// MissileSecondsPerTile turns a D1 missile velocity (GetMissileVel: screen pixels a game tick, 20 ticks a second)
// into seconds per tile: one step along a grid axis is 32·√2 ≈ 45.25 of those pixels.
func MissileSecondsPerTile(v int) float64 { return 45.25 / 20 / float64(v) }

// SpellMissileVel is the velocity AddMissile gives the hero's travelling spells.
func SpellMissileVel(spell, slvl int) int {
	switch spell {
	case SplFirebolt, SplHolyBolt:
		return 16 + min(2*slvl, 47)
	case SplFireball:
		return 16 + min(2*slvl, 34)
	case SplChargedBolt:
		return 8
	case SplLightning, SplChain:
		return 32
	}
	return 16 // Elemental, Bone Spirit, Blood Star, Flame Wave
}

// ChargedBoltFlight ports MI_Cbolt's wander: every 17 ticks the spark turns to dir plus a bpath step (from rnd, which
// AddCbolt rolls 1-15) and flies at velocity 8 for its 256 ticks. It returns the spark's tile relative to the caster
// after each tick (GetMissilePos).
func ChargedBoltFlight(dir, rnd int) [][2]int {
	bpath := [16]int{-1, 0, 1, -1, 0, 1, -1, -1, 0, 0, 1, 1, 0, 1, -1, 0}
	xAdd, yAdd := [8]int{1, 0, -1, -1, -1, 0, 1, 1}, [8]int{1, 1, 1, 0, -1, -1, -1, 0}
	var tx, ty, xvel, yvel, wait int
	out := make([][2]int, 256)
	for i := range out {
		if wait == 0 {
			md := (dir + bpath[rnd]) & 7
			rnd = (rnd + 1) & 15
			xvel, yvel = missileVel(xAdd[md], yAdd[md], 8)
			wait = 16
		} else {
			wait--
		}
		tx, ty = tx+xvel, ty+yvel
		out[i] = missilePos(tx, ty)
	}
	return out
}

// MissileFlight is where a missile flying from the origin towards (dx, dy) at velocity v is after each of its
// `ticks` ticks, relative to the origin (GetMissileVel, GetMissilePos).
func MissileFlight(dx, dy, v, ticks int) [][2]int {
	xvel, yvel := missileVel(dx, dy, v)
	out := make([][2]int, ticks)
	for i := range out {
		out[i] = missilePos(xvel*(i+1), yvel*(i+1))
	}
	return out
}

// missileVel ports GetMissileVel.
func missileVel(dx, dy, v int) (int, int) {
	if dx == 0 && dy == 0 {
		return 0, 0
	}
	dxp, dyp := float64((dx-dy)<<21), float64((dx+dy)<<21)
	dr := math.Sqrt(dxp*dxp + dyp*dyp)
	return int(dxp * float64(v<<16) / dr), int(dyp * float64(v<<15) / dr)
}

// missilePos ports GetMissilePos's tile, D1 rounding toward zero.
func missilePos(tx, ty int) [2]int {
	shift := func(v int) int {
		if v < 0 {
			return -(-v >> 6)
		}
		return v >> 6
	}
	mx, my := tx>>16, ty>>16
	return [2]int{shift(mx + 2*my), shift(2*my - mx)}
}

// GuardianScan is the order MI_Guardian tries cells in (Sentfire): each VCrawlTable ray from its 6th point inwards,
// every point in four mirrored quadrants, skipping a point equal to the one just tried.
var GuardianScan = func() (out [][2]int) {
	var last [2]int
	for j := range VCrawlTable {
		for k := 10; k >= 0 && (VCrawlTable[j][k] != 0 || VCrawlTable[j][k+1] != 0); k -= 2 {
			x, y := VCrawlTable[j][k], VCrawlTable[j][k+1]
			if last == [2]int{x, y} {
				continue
			}
			out = append(out, [2]int{x, y}, [2]int{-x, -y}, [2]int{x, -y}, [2]int{-x, y})
			last = [2]int{x, y}
		}
	}
	return
}()

// Whole converts D1's 1/64 hit points, mana and damage to the whole points this engine's resources hold. It is
// the one place the two units meet: nothing outside this package shifts by 6 to do it by hand.
func Whole(v64 int) int { return v64 >> 6 }

// Fixed is Whole's inverse: whole points as D1's 1/64.
func Fixed(whole int) int { return whole << 6 }
