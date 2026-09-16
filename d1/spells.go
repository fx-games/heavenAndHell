package d1

import "math/rand"

// Spell ids (SPL_) of the 26 single player spells (decision 6).
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
)

// Missile resistance kinds (missiledata mResist).
const (
	ResistNone = iota
	ResistKindFire
	ResistKindLightning
	ResistKindMagic
)

// SpellResist is the resistance kind of each spell's damaging missile.
var SpellResist = map[int]int{
	SplFirebolt: ResistKindFire, SplFireWall: ResistKindFire, SplFireball: ResistKindFire, SplGuardian: ResistKindFire,
	SplWave: ResistKindFire, SplInferno: ResistKindFire, SplElemental: ResistKindFire,
	SplLightning: ResistKindLightning, SplChain: ResistKindLightning, SplNova: ResistKindLightning, SplChargedBolt: ResistKindLightning,
	SplFlash: ResistKindMagic, SplStone: ResistKindMagic, SplApocalypse: ResistNone, SplBloodStar: ResistKindMagic,
	SplBoneSpirit: ResistKindMagic, SplHolyBolt: ResistNone,
}

// SpellTargeted: spells that need a target cell (sTargeted, plus the ones D1 aims at the cursor anyway).
var SpellTargeted = map[int]bool{
	SplFirebolt: true, SplLightning: true, SplFireWall: true, SplStone: true, SplFireball: true, SplGuardian: true,
	SplChain: true, SplWave: true, SplInferno: true, SplGolem: true, SplTeleport: true, SplElemental: true,
	SplChargedBolt: true, SplHolyBolt: true, SplTelekinesis: true, SplBloodStar: true, SplBoneSpirit: true,
}

// SpellFrames: casting animation length per class (PlrGFXAnimLens[class][5]).
var SpellFrames = [3]int{20, 16, 12}

// SpellLevel ports GetSpellLevel.
func (h *Hero) SpellLevel(s Sheet, spell int) int {
	return max(h.SpellLevels[spell]+s.SplLvlAdd, 0)
}

// ManaCost ports GetManaAmount (1/64).
func (d *Data) ManaCost(h *Hero, s Sheet, spell int) int {
	sd := d.Spells[spell]
	sl := max(h.SpellLevels[spell]+s.SplLvlAdd-1, 0)
	adj := sl * sd.ManaAdj
	if spell == SplFirebolt {
		adj >>= 1
	}
	ma := (sd.Mana - adj) << 6
	if spell == SplHeal {
		ma = (sd.Mana + 2*h.Level - adj) << 6
	}
	if h.Class == Rogue {
		ma -= ma >> 2
	}
	if sd.MinMana > ma>>6 {
		ma = sd.MinMana << 6
	}
	return ma
}

// SpellHitChance ports MonsterMHit's to-hit for magic missiles.
func (h *Hero) SpellHitChance(s Sheet, mlvl, dist int) int {
	hper := s.Mag + 50 - mlvl<<1 - dist
	if h.Class == Sorcerer {
		hper += 20
	}
	return clamp(hper, 5, 95)
}

func rnd(rng *rand.Rand, n int) int {
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

func FireboltDamage(rng *rand.Rand, mag, slvl int) int { return mag>>3 + rnd(rng, 10) + slvl + 1 }
func BloodStarDamage(mag, slvl int) int                { return mag>>1 + 3*slvl - mag>>3 }
func LightningDamage(rng *rand.Rand, clvl int) int     { return rnd(rng, 2) + rnd(rng, clvl) + 2 }
func HolyBoltDamage(rng *rand.Rand, clvl int) int      { return rnd(rng, 10) + clvl + 9 }
func ChargedBoltDamage(rng *rand.Rand, mag int) int    { return rnd(rng, mag>>2) + 1 }
func ChargedBolts(slvl int) int                        { return slvl>>1 + 4 }
func WaveDamage(rng *rand.Rand, clvl int) int          { return rnd(rng, 10) + clvl + 1 }

func FireballDamage(rng *rand.Rand, clvl, slvl int) int {
	return grow(2*(rnd(rng, 10)+rnd(rng, 10)+clvl+2), slvl)
}

func ElementalDamage(rng *rand.Rand, clvl, slvl int) int { return FireballDamage(rng, clvl, slvl) >> 1 }

func NovaDamage(rng *rand.Rand, clvl, slvl int) int {
	d := 0
	for i := 0; i < 5; i++ {
		d += rnd(rng, 6)
	}
	return grow((d+clvl+5)>>1, slvl)
}

func ApocalypseDamage(rng *rand.Rand, clvl int) int {
	d := 0
	for i := 0; i < clvl; i++ {
		d += rnd(rng, 6) + 1
	}
	return d
}

// FlashDamage64 returns the per-tick damage of the flash below and around the caster, and of the flash above.
func FlashDamage64(rng *rand.Rand, clvl, slvl int) (low, high int) {
	for i := 0; i <= clvl; i++ {
		low += rnd(rng, 20) + 1
		high += rnd(rng, 2) + 1
	}
	low, high = grow(low, slvl), grow(high, slvl)
	return low + low>>1, high + high>>1
}

const FlashTicks = 19

func FireWallDamage64(rng *rand.Rand, clvl int) int {
	return 16 * (rnd(rng, 10) + rnd(rng, 10) + clvl + 2) >> 1
}

// FireWallTurns ports AddFirewall's range (ticks) in turns.
func FireWallTurns(slvl int) int { return 10 * (slvl + 1) * 16 / RefWalkTicks }

func InfernoDamage64(rng *rand.Rand, clvl int) int {
	d := 8 * (rnd(rng, 2) + rnd(rng, clvl) + 2)
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

// HealAmount64 ports AddHeal.
func HealAmount64(rng *rand.Rand, class, clvl, slvl int) int {
	l := (rnd(rng, 10) + 1) << 6
	for i := 0; i < clvl; i++ {
		l += (rnd(rng, 4) + 1) << 6
	}
	for i := 0; i < slvl; i++ {
		l += (rnd(rng, 6) + 1) << 6
	}
	switch class {
	case Warrior:
		l *= 2
	case Rogue:
		l += l >> 1
	}
	return l
}

// HealRange64 is HealAmount64's lowest and highest roll.
func HealRange64(class, clvl, slvl int) (int, int) {
	lo, hi := (1+clvl+slvl)<<6, (10+4*clvl+6*slvl)<<6
	switch class {
	case Warrior:
		return lo * 2, hi * 2
	case Rogue:
		return lo + lo>>1, hi + hi>>1
	}
	return lo, hi
}

// Golem ports SpawnGolum's stats; maxMana in 1/64, HP returned in whole points.
type Golem struct{ HP, AC, Hit, MinDam, MaxDam int }

func GolemStats(slvl, clvl, maxMana int) Golem {
	return Golem{HP: 2 * (320*slvl + maxMana/3) >> 6, AC: 25, Hit: 5*(slvl+8) + 2*clvl, MinDam: 2 * (slvl + 4), MaxDam: 2 * (slvl + 8)}
}

// NovaRays are the offsets MI_Nova sends lightning balls towards (vCrawlTable[k][6..7], mirrored).
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
