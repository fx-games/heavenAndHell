package common

import (
	"fmt"
	"math/rand"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/util"
)

// spell_damage is the one damaging on-hit effect. Everything a damaging spell used to differ in - how far and in
// what shape it reaches - is now its hit zone; what is left is a damage roll named by the content, the resistance
// it is checked against, and whether it only harms the undead or drains a share of what the target has left.
//
//	effect: spell_damage(<roll>, <resist>[, <mode>])
//
// The caster's spell level is appended as a fourth argument where the action is built, because a roll grows with
// it. <roll> names an entry of GetAllDamageRolls, <resist> is none/fire/lightning/magic, and <mode> is empty,
// undead_only or drain_third.

// RollContext is everything a damage roll may read: the caster's magic, character level and this spell's level.
type RollContext struct {
	Rng             *rand.Rand
	Mag, Clvl, Slvl int
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
			return SpellRoll{Damage64: d1.InfernoDamage64(c.Rng, c.Clvl), Ticks: d1.InfernoTicks(0)}
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

var damageIconByResist = map[int]int32{
	d1.ResistKindFire: defs.IconEffectFireSmall, d1.ResistKindLightning: defs.IconEffectLightning,
	d1.ResistNone: defs.IconEffectHit, d1.ResistKindMagic: defs.IconEffectHit,
}

func (g *Factory) spellDamage(args util.Arguments) *HitEffect {
	rollName := args.Get(0)
	if _, known := GetAllDamageRolls()[rollName]; !known {
		g.Msg(fmt.Sprintf("Unknown spell damage roll: %s", rollName))
		return nil
	}
	resist := resistKindByName[args.Get(1)]
	mode := args.Get(2)
	slvl := args.GetInt(3)
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
	animations   []Animation
	landed       bool
}

func NewSpellDamageAction(caster *Actor, roll string, resist int, mode string, slvl int, target geometry.Point, hitPositions []geometry.Point) *SpellDamageAction {
	return &SpellDamageAction{
		BaseAction:   BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
		caster:       caster, roll: roll, resist: resist, mode: mode, slvl: slvl,
		target: target, hitPositions: hitPositions,
	}
}

func (a *SpellDamageAction) GetAnimations() []Animation { return a.animations }
func (a *SpellDamageAction) GetDelayInAut() int         { return 0 }

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
	roll := GetAllDamageRolls()[a.roll](RollContext{Rng: d1Rng, Mag: c.s.Mag, Clvl: c.h.Level, Slvl: a.slvl})
	icon := damageIconByResist[a.resist]
	for _, p := range a.hitPositions {
		if victim := c.hitCell(p); victim != nil && c.strike(victim, roll.Damage64, roll.Ticks) {
			a.landed = true
		}
	}
	if len(a.hitPositions) > 0 {
		last := a.hitPositions[len(a.hitPositions)-1]
		a.animations = append(c.animations, NewProjectileDirectionAnimation(a.caster, a.caster.MapPosition(), last, 0.03, icon, 2))
	}
	return c.finish()
}
