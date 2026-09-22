package common

import (
	"fmt"
	"math/rand"
	"rpg/src/d1"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/stats"
	"rpg/src/util"
	"strings"
)

// spell_damage is the one damaging on-hit effect. Everything a damaging spell used to differ in - how far and in
// what shape it reaches - is now its hit zone; what is left is a damage roll named by the content, the resistance
// it is checked against, and whether it only harms the undead or drains a share of what the target has left.
//
//	effect: spell_damage(<roll>, <resist>[, <mode>])
//
// The caster's spell level is appended as a fourth argument where the action is built, because a roll grows with
// it. <roll> names an entry of GetAllDamageRolls, <resist> is none/fire/lightning/magic, and <mode> is empty,
// undead_only, drain_third or per_cell. per_cell rolls afresh for every cell and tells the roll which cell it is,
// for spells that lay one missile per cell (Lightning, Inferno) rather than one blast.

// RollContext is everything a damage roll may read: the caster's magic, character level and this spell's level.
type RollContext struct {
	Rng             *rand.Rand
	Mag, Clvl, Slvl int
	Cell            int // per_cell: the cell's place along the hit zone, from 0
}

// SpellRoll is one roll's outcome: damage in 1/64 points, and how many times it is rolled against the victim.
// A spell that burns a cell over several ticks rolls once and hits Ticks times, the way D1's lingering missiles do.
type SpellRoll struct {
	Damage64, Ticks int
}

func whole(damage, ticks int) SpellRoll { return SpellRoll{Damage64: damage << 6, Ticks: ticks} }

// GetAllDamageRolls is the damage a spell deals, by name. The arithmetic lives in the d1 tables package with the
// rest of the numbers; this map is only the name each roll answers to, the same way hit zones and targeting rules
// are reached by name.
func GetAllDamageRolls() map[string]func(RollContext) SpellRoll {
	return map[string]func(RollContext) SpellRoll{
		"firebolt":     func(c RollContext) SpellRoll { return whole(d1.FireboltDamage(c.Rng, c.Mag, c.Slvl), 1) },
		"holy_bolt":    func(c RollContext) SpellRoll { return whole(d1.HolyBoltDamage(c.Rng, c.Clvl), 1) },
		"blood_star":   func(c RollContext) SpellRoll { return whole(d1.BloodStarDamage(c.Mag, c.Slvl), 1) },
		"charged_bolt": func(c RollContext) SpellRoll { return whole(d1.ChargedBoltDamage(c.Rng, c.Mag), 1) },
		"wave":         func(c RollContext) SpellRoll { return whole(d1.WaveDamage(c.Rng, c.Clvl), 1) },
		"fireball":     func(c RollContext) SpellRoll { return whole(d1.FireballDamage(c.Rng, c.Clvl, c.Slvl), 1) },
		"elemental":    func(c RollContext) SpellRoll { return whole(d1.ElementalDamage(c.Rng, c.Clvl, c.Slvl), 1) },
		"nova":         func(c RollContext) SpellRoll { return whole(d1.NovaDamage(c.Rng, c.Clvl, c.Slvl), 1) },
		"apocalypse":   func(c RollContext) SpellRoll { return whole(d1.ApocalypseDamage(c.Rng, c.Clvl), 1) },
		"bone_spirit":  func(c RollContext) SpellRoll { return whole(0, 1) }, // drain_third supplies the amount
		"lightning": func(c RollContext) SpellRoll {
			return whole(d1.LightningDamage(c.Rng, c.Clvl), d1.LightningTicks(c.Slvl))
		},
		"inferno": func(c RollContext) SpellRoll {
			return SpellRoll{Damage64: d1.InfernoDamage64(c.Rng, c.Clvl), Ticks: d1.InfernoTicks(c.Cell)}
		},
		"flash": func(c RollContext) SpellRoll {
			low, _ := d1.FlashDamage64(c.Rng, c.Clvl, c.Slvl)
			return SpellRoll{Damage64: low, Ticks: d1.FlashTicks}
		},
	}
}

var resistKindByName = map[string]int{
	"none": d1.ResistNone, "fire": d1.ResistKindFire, "lightning": d1.ResistKindLightning, "magic": d1.ResistKindMagic,
}

func damageIconByResist() map[int]int32 {
	return map[int]int32{
		d1.ResistKindFire: defs.IconEffectFireSmall, d1.ResistKindLightning: defs.IconEffectLightning,
		d1.ResistNone: defs.IconEffectHit, d1.ResistKindMagic: defs.IconEffectHit,
	}
}

// spellDamageModeAndLevel reads spell_damage's optional mode and the spell level the cast appends after it.
func spellDamageModeAndLevel(args util.Arguments) (mode string, slvl int) {
	if len(args) > 3 {
		mode = args.Get(2)
	}
	return mode, args.GetInt(len(args) - 1)
}

func (g *Factory) spellDamage(args util.Arguments) *HitEffect {
	rollName := args.Get(0)
	if _, known := GetAllDamageRolls()[rollName]; !known {
		g.Msg(fmt.Sprintf("Unknown spell damage roll: %s", rollName))
		return nil
	}
	resist := resistKindByName[args.Get(1)]
	mode, slvl := spellDamageModeAndLevel(args)
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			g.ActionBroker.Execute(NewSpellDamageAction(attacker, rollName, resist, mode, slvl, target, hitPositions))
		},
		Description: []util.LabelText{util.LabelTextFromString("Damage every target in the spell's reach")},
	}
}

// SpellDamageAction rolls a spell's damage and applies it to everything standing in its hit positions, with the
// resistance check, the to-hit roll and the once-per-victim accumulation the combat model uses.
type SpellDamageAction struct {
	BaseAction
	caster       *Actor
	roll         string
	resist       int
	mode         string
	slvl         int
	target       geometry.Point
	hitPositions []geometry.Point
	landed       bool
}

func NewSpellDamageAction(caster *Actor, roll string, resist int, mode string, slvl int, target geometry.Point, hitPositions []geometry.Point) *SpellDamageAction {
	return &SpellDamageAction{
		BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
		caster:     caster, roll: roll, resist: resist, mode: mode, slvl: slvl,
		target: target, hitPositions: hitPositions,
	}
}

func (a *SpellDamageAction) GetDelayInAut() int { return 0 }

// Succeeded: the spell found something to act on. A spell that did nothing costs nothing.
func (a *SpellDamageAction) Succeeded() bool { return a.landed }

func (a *SpellDamageAction) Execute(ctx ActionContext) []Action {
	f := ctx.GameState.factory
	c := &spellCtx{f: f, ctx: ctx, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), slvl: a.slvl,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt: map[*Actor]int{},
		resist: a.resist, undeadOnly: a.mode == "undead_only", drainThird: a.mode == "drain_third"}
	if c.h == nil {
		return nil
	}
	rollAt := func(cell int) SpellRoll {
		return GetAllDamageRolls()[a.roll](RollContext{Rng: rng, Mag: c.s.Mag, Clvl: c.h.Level, Slvl: a.slvl, Cell: cell})
	}
	roll := rollAt(0)
	icon := damageIconByResist()[a.resist]
	for i, p := range a.hitPositions {
		if a.mode == "per_cell" && i > 0 {
			roll = rollAt(i)
		}
		if victim := c.hitCell(p); victim != nil && c.strike(victim, roll.Damage64, roll.Ticks) {
			a.landed = true
		}
	}
	if len(a.hitPositions) > 0 {
		last := a.hitPositions[len(a.hitPositions)-1]
		c.missile(a.caster, []geometry.Point{a.caster.MapPosition(), last}, 0.03, icon)
		c.emit(ctx.ActionBroker) // a spell that hit no cell shows nothing
	}
	return c.finish()
}

// charged_bolts(<roll>, <resist>) fires several sparks that each wander off towards the target on their own and
// stop at the first thing they hit, rolling <roll> separately. It is its own effect rather than a hit zone
// because how many sparks fly is the caster's spell level, which a zone never sees, and because the union of
// their paths would lose what makes the spell the spell: each spark spends itself on one victim.
// ponytail: multi_shot is the nearest thing already in the untargeted registry, but it shoots the equipped bow
// at the nearest enemies, so nothing of it carries over but the idea.
func (g *Factory) chargedBolts(args util.Arguments) *HitEffect {
	rollName := args.Get(0)
	if _, known := GetAllDamageRolls()[rollName]; !known {
		g.Msg(fmt.Sprintf("Unknown spell damage roll: %s", rollName))
		return nil
	}
	resist := resistKindByName[args.Get(1)]
	slvl := args.GetInt(2)
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			g.ActionBroker.Execute(&ChargedBoltsAction{
				BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, attacker), name: ActionNameFireball},
				caster:     attacker, roll: rollName, resist: resist, slvl: slvl, target: target})
		},
		Description: []util.LabelText{util.LabelTextFromString("Release wandering sparks towards the target")},
	}
}

// ChargedBoltsAction is charged_bolts resolving: d1.ChargedBolts(slvl) sparks, each with its own wander seed,
// its own damage roll and its own path animation.
type ChargedBoltsAction struct {
	BaseAction
	caster *Actor
	roll   string
	resist int
	slvl   int
	target geometry.Point
	landed bool
}

func (a *ChargedBoltsAction) GetDelayInAut() int { return 0 }
func (a *ChargedBoltsAction) Succeeded() bool    { return a.landed }

func (a *ChargedBoltsAction) Execute(ctx ActionContext) []Action {
	f := ctx.GameState.factory
	c := &spellCtx{f: f, ctx: ctx, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), slvl: a.slvl,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt: map[*Actor]int{}, resist: a.resist}
	if c.h == nil {
		return nil
	}
	origin := a.caster.MapPosition()
	for n := d1.ChargedBolts(a.slvl); n > 0; n-- {
		wander := rng.Intn(15) + 1 // drawn before the damage, the order the original missile used
		roll := GetAllDamageRolls()[a.roll](RollContext{Rng: rng, Mag: c.s.Mag, Clvl: c.h.Level, Slvl: a.slvl})
		p, path := origin, []geometry.Point{origin}
		for _, next := range c.chargedBoltCells(origin, a.target, wander) {
			p = next
			path = append(path, p)
			if victim := c.hitCell(p); victim != nil && c.strike(victim, roll.Damage64, roll.Ticks) {
				a.landed = true
				break
			}
		}
		if p != origin {
			c.missile(a.caster, path, 0.03, defs.IconEffectLightning)
		}
	}
	c.emit(ctx.ActionBroker)
	return c.finish()
}

// restore_resource is spell_damage's mirror: the one effect that puts a resource back, on the caster.
//
//	effect: restore_resource(<health|mana>, <base>, <per_clvl>, <per_slvl>, <doubled_for>, <raised_by_half_for>)
//	effect: restore_resource(<health|mana>, full)
//
// Healing, Mana and the Magi are the same curve read three ways, so the dice and the classes each one favours
// are numbers in the content and the arithmetic sits in d1.ResourceAmount64 beside the damage rolls. The
// caster's spell level is appended as the last argument where the action is built, as it is for spell_damage.
// classSet reads a comma separated list of class keys (hero.rec's key). Commas, because the argument list itself splits on '|'.
func classSet(d *d1.Data, list string) map[int]bool {
	set := map[int]bool{}
	for _, name := range strings.Split(list, ",") {
		if class := d.ClassByKey(strings.TrimSpace(name)); class >= 0 {
			set[class] = true
		}
	}
	return set
}

func (g *Factory) restoreResource(args util.Arguments) *HitEffect {
	resource := args.Get(0)
	if resource != "health" && resource != "mana" {
		g.Msg(fmt.Sprintf("Unknown resource to restore: %s", resource))
		return nil
	}
	full := args.Get(1) == "full"
	base, perClvl, perSlvl := args.GetInt(1), args.GetInt(2), args.GetInt(3)
	doubled, raisedByHalf := classSet(g.Generator.D, args.Get(4)), classSet(g.Generator.D, args.Get(5))
	slvl := args.GetInt(len(args) - 1) // the cast appends it, and the two forms are of different length
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			h := attacker.hero
			if h == nil {
				return
			}
			boost := 0
			switch {
			case doubled[h.Class]:
				boost = 2
			case raisedByHalf[h.Class]:
				boost = 1
			}
			st := attacker.GetStats()
			if resource == "health" {
				if full {
					st.GetResource(stats.Health).SetToMax()
					return
				}
				st.GetResource(stats.Health).Increase(float64(d1.ResourceAmount64(rng, base, perClvl, perSlvl, h.Level, slvl, boost) >> 6))
				return
			}
			// mana carries hero bookkeeping health does not: the base pool grows with what is restored
			if full {
				h.ManaBase = h.MaxManaBase
				st.GetResource(stats.Mana).SetToMax()
				return
			}
			amount := d1.ResourceAmount64(rng, base, perClvl, perSlvl, h.Level, slvl, boost) >> 6
			h.ManaBase = min(h.ManaBase+amount<<6, h.MaxManaBase)
			st.GetResource(stats.Mana).Increase(float64(amount))
		},
		Description: []util.LabelText{util.LabelTextFromString(fmt.Sprintf("Restore the caster's %s", resource))},
	}
}
