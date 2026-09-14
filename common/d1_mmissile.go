package common

import (
	"fmt"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/stats"
)

// D1Missile is a missile a monster fires (MIS_ ids of monster attacks). Arrows go through the ranged attack action.
type D1Missile int

const (
	MisFirebolt   D1Missile = iota // counselors
	MisFireball                    // magistrates
	MisLightCtrl                   // cabalists: a lightning line
	MisLightCtrl2                  // storm demons
	MisLightning                   // familiars: a strike on the enemy
	MisFlash                       // counselors in melee
	MisMagmaball
	MisFlare  // succubi (Blood Star)
	MisFlamec // balrogs: three flames towards the enemy
	MisCbolt  // advocates: three charged bolts
	MisAcid   // acid beasts, leaves a puddle
	MisAcidPud
)

// d1MissileResist is missiledata's mResist for these missiles; all of them are magical (mType 1).
var d1MissileResist = map[D1Missile]int{
	MisFirebolt: d1.ResistKindFire, MisFireball: d1.ResistKindFire, MisLightCtrl: d1.ResistKindLightning,
	MisLightCtrl2: d1.ResistKindLightning, MisLightning: d1.ResistKindLightning, MisFlash: d1.ResistKindMagic,
	MisMagmaball: d1.ResistKindFire, MisFlare: d1.ResistKindMagic, MisFlamec: d1.ResistKindFire,
	MisCbolt: d1.ResistKindLightning, MisAcid: d1.ResistKindMagic, MisAcidPud: d1.ResistKindMagic,
}

// MonsterMissileAction resolves one monster missile at once on the grid, like the player's spells (decision 6):
// bolts fly until they hit the player or a wall, lines and flashes roll D1's per-tick hits in one go.
type MonsterMissileAction struct {
	BaseAction
	caster     *Actor
	mis        D1Missile
	target     geometry.Point
	dam        int
	animations []Animation
}

func NewMonsterMissileAction(caster *Actor, mis D1Missile, target geometry.Point, dam int) *MonsterMissileAction {
	return &MonsterMissileAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameMonsterMissile, caster), name: ActionNameMonsterMissile},
		caster: caster, mis: mis, target: target, dam: dam}
}

func (a *MonsterMissileAction) GetAnimations() []Animation { return a.animations }

func (a *MonsterMissileAction) Execute(ctx ActionContext) []Action {
	c := &missileCtx{ctx: ctx, caster: a.caster, mon: a.caster.d1Monster, player: ctx.GameState.GetPlayer(), c: &spellCtx{m: ctx.GameState.GetMap(), caster: a.caster}}
	c.fire(a.mis, a.target, a.dam)
	a.animations = c.c.animations
	if c.dealt <= 0 || !c.player.IsAlive() {
		return nil
	}
	return OneAction(NewChangeResourceAction(a.cause, c.player, stats.Health, -float64(c.dealt)))
}

type missileCtx struct {
	ctx    ActionContext
	caster *Actor
	mon    *D1Monster
	player *Actor
	c      *spellCtx // for its grid helpers
	dealt  int       // whole points
}

// hit ports PlayerMHit: `ticks` hit rolls against the player standing at p, from a missile that flew from `from`.
func (c *missileCtx) hit(mis D1Missile, p, from geometry.Point, mind, maxd int, shift bool, ticks int) bool {
	pl := c.player
	if pl.hero == nil || !pl.IsAlive() || pl.MapPosition() != p {
		return false
	}
	h, s := pl.hero, pl.HeroSheet()
	chance := h.MonsterSpellHitChance(c.mon.Level, geometry.DistanceChebyshev(from, p), d1DungeonLevel(c.ctx.GameState))
	resper := map[int]int{d1.ResistKindFire: s.FR, d1.ResistKindLightning: s.LR, d1.ResistKindMagic: s.MR}[d1MissileResist[mis]]
	block := 0
	if !shift && mis != MisAcidPud {
		block = h.BlockChance(s, c.mon.Level)
	}
	total, blocked := 0, false
	for i := 0; i < ticks; i++ {
		if d1Rng.Intn(100) >= chance {
			continue
		}
		dam := d1.MissileDamage64(d1Rng, s, mind, maxd, shift)
		if resper > 0 {
			dam -= resper * dam / 100
		} else if d1Rng.Intn(100) < block {
			blocked = true
			continue
		}
		total += dam
	}
	if blocked && total == 0 {
		c.ctx.Msg(fmt.Sprintf("You block the attack of %s.", c.caster.Name()))
	}
	if total == 0 {
		return false
	}
	c.dealt += d1ManaShield(pl, total>>6)
	c.c.animations = append(c.c.animations, NewHitAnimation(p, 0.3, defs.IconEffectHit, 2))
	return true
}

func (c *missileCtx) monsterDamage() int {
	return c.mon.MinDam + d1Rng.Intn(max(c.mon.MaxDam-c.mon.MinDam+1, 1))
}

func missileIcon(mis D1Missile) int32 {
	switch d1MissileResist[mis] {
	case d1.ResistKindFire:
		return defs.IconEffectFireSmall
	case d1.ResistKindLightning:
		return defs.IconEffectLightning
	}
	return defs.IconEffectHit
}

// bolt flies towards the target until it hits the player or something solid (MI_Firebolt, CheckMissileCol).
func (c *missileCtx) bolt(mis D1Missile, to geometry.Point, dam int) (geometry.Point, bool) {
	from, last, hit := c.caster.MapPosition(), c.caster.MapPosition(), false
	for _, p := range c.c.ray(from, to, 20) {
		if o := d1ObjectAt(c.c.m, p); o != nil && !o.IsWalkable(nil) { // monster missiles fly through other monsters
			break
		}
		last = p
		if hit = c.hit(mis, p, from, dam, dam, false, 1); hit {
			break
		}
	}
	if last != from {
		c.c.animations = append(c.c.animations, NewProjectileDirectionAnimation(c.caster, from, last, 0.03, missileIcon(mis), 2))
	}
	return last, hit
}

func (c *missileCtx) fire(mis D1Missile, target geometry.Point, dam int) {
	o := c.caster.MapPosition()
	switch mis {
	case MisFirebolt, MisMagmaball, MisFlare:
		c.bolt(mis, target, c.monsterDamage())
	case MisAcid:
		if end, _ := c.bolt(mis, target, c.monsterDamage()); end != o { // MI_Acidsplat
			d := 1
			if c.mon.Data.Level >= 2 {
				d = 2
			}
			c.hazard(MisAcidPud, end, (d1Rng.Intn(15)+40*(c.mon.Int+1))/d1.RefWalkTicks, d)
		}
	case MisCbolt:
		for i := 0; i < 3; i++ { // ponytail: charged bolts fly straight instead of D1's wobbling paths
			c.bolt(mis, target, 15)
		}
	case MisFireball:
		end, _ := c.bolt(mis, target, dam)
		if p := c.player.MapPosition(); geometry.DistanceChebyshev(end, p) <= 1 && c.c.lineClear(end, p) {
			c.hit(mis, p, o, dam, dam, false, 1)
		}
	case MisLightCtrl, MisLightCtrl2: // a lightning on every cell of the line, each hitting for 10 ticks
		cells := c.c.ray(o, target, 20)
		for _, p := range cells {
			if c.hit(mis, p, o, 2*c.monsterDamage(), 2*c.monsterDamage(), true, 10) {
				break
			}
		}
		if len(cells) > 0 {
			c.c.animations = append(c.c.animations, NewProjectileDirectionAnimation(c.caster, o, cells[len(cells)-1], 0.01, missileIcon(mis), 2))
		}
	case MisLightning:
		c.hit(mis, target, target, dam, dam, true, 8)
		c.c.animations = append(c.c.animations, NewHitAnimation(target, 0.3, defs.IconEffectLightning, 2))
	case MisFlash: // the six cells from one left of the caster to one right and below
		for _, d := range []geometry.Point{{X: -1}, {}, {X: 1}, {X: -1, Y: 1}, {Y: 1}, {X: 1, Y: 1}} {
			c.hit(mis, o.Add(d), o, 2*c.mon.Level, 2*c.mon.Level, true, d1.FlashTicks)
		}
	case MisFlamec: // MI_Flamec: flames on the next three cells towards the target, the n-th burning 20+5n ticks
		for n, p := range c.c.ray(o, target, 3) {
			c.hazard(MisFlamec, p, (20+5*n)/d1.RefWalkTicks, c.monsterDamage())
		}
	}
}

// hazard leaves a burning flame or acid puddle that hits the player on it every turn (8 ticks, 1/64 damage each).
func (c *missileCtx) hazard(mis D1Missile, p geometry.Point, turns, dam64 int) {
	f := c.ctx.GameState.factory
	m := c.c.m
	if turns <= 0 || m.IsObjectAt(p) || !m.IsWalkable(p) {
		return
	}
	caster, mon := c.caster, c.mon
	hz := NewFire(0)
	hz.turnsUntilRemoval = turns
	if mis == MisAcidPud {
		hz.icon = defs.IconEffectHit
	}
	burn := func() {
		if f.state.GetMap() != m || c.player.MapPosition() != p {
			return
		}
		f.actionBroker.Execute(&hazardTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			mc: &missileCtx{caster: caster, mon: mon}, mis: mis, p: p, dam64: dam64})
	}
	hz.SetDealDamage(func(geometry.Point, func() bool) {})
	hz.SetOnTurn(func() {
		if !hz.IsAlive() || f.state.GetMap() != m {
			m.RemoveObject(hz)
			hz.turnsUntilRemoval = 0
			return
		}
		burn()
	})
	f.getTimeKeeper().AddTimed(hz)
	m.AddObject(hz, p)
	burn()
}

type hazardTickAction struct {
	BaseAction
	mc         *missileCtx
	mis        D1Missile
	p          geometry.Point
	dam64      int
	animations []Animation
}

func (a *hazardTickAction) GetAnimations() []Animation { return a.animations }
func (a *hazardTickAction) Execute(ctx ActionContext) []Action {
	c := a.mc
	c.ctx, c.player, c.c, c.dealt = ctx, ctx.GameState.GetPlayer(), &spellCtx{m: ctx.GameState.GetMap(), caster: c.caster}, 0
	c.hit(a.mis, a.p, a.p, a.dam64, a.dam64, true, d1.RefWalkTicks)
	a.animations = c.c.animations
	if c.dealt <= 0 {
		return nil
	}
	return OneAction(NewChangeResourceAction(a.cause, c.player, stats.Health, -float64(c.dealt)))
}

// D1AcidDeath: acid beasts leave a puddle where they die (M_StartKill).
func (f *Factory) d1AcidDeath(ctx ActionContext, victim *Actor) {
	if mon := victim.d1Monster; mon.Data.Family == "Acid" {
		c := &missileCtx{ctx: ctx, caster: victim, mon: mon, player: ctx.GameState.GetPlayer(), c: &spellCtx{m: ctx.GameState.GetMap(), caster: victim}}
		p := victim.MapPosition()
		c.hazard(MisAcidPud, p, (d1Rng.Intn(15)+40*(mon.Int+1))/d1.RefWalkTicks, mon.Int+1)
	}
}
