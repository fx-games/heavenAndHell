package common

import (
	"fmt"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/rules"
	"rpg/src/stats"
)

// MonsterMissile is a missile a monster fires (MIS_ ids of monster attacks). Arrows go through the ranged attack action.
type MonsterMissile int

const (
	MisFirebolt   MonsterMissile = iota // counselors
	MisFireball                         // magistrates
	MisLightCtrl                        // cabalists: a lightning line
	MisLightCtrl2                       // storm demons
	MisLightning                        // familiars: a strike on the enemy
	MisFlash                            // counselors in melee
	MisMagmaball
	MisFlare  // succubi (Blood Star)
	MisFlamec // balrogs: three flames towards the enemy
	MisCbolt  // advocates: three charged bolts
	MisAcid   // acid beasts, leaves a puddle
	MisAcidPud
	MisApoca  // Diablo: a blast on the player if he sees them
	MisArrow  // traps: a plain arrow
	MisFarrow // trapped chests: a fire arrow that bursts where it stops
	MisNova   // trapped chests: a ring of lightning balls
	// Hellfire's
	MisFireRing  // trapped chests: fire walls three cells out
	MisFireWall  // the Oily shrine: a fire wall sent at the hero
	MisFlashTrap // the Sparkling shrine: a flash around the shrine
)

// missileIDs are the missiles.rec ids of these missiles (misdat's MIS_ names).
var missileIDs = [...]string{MisFirebolt: "MIS_FIREBOLT", MisFireball: "MIS_FIREBALL", MisLightCtrl: "MIS_LIGHTCTRL",
	MisLightCtrl2: "MIS_LIGHTCTRL2", MisLightning: "MIS_LIGHTNING", MisFlash: "MIS_FLASH", MisMagmaball: "MIS_MAGMABALL",
	MisFlare: "MIS_FLARE", MisFlamec: "MIS_FLAMEC", MisCbolt: "MIS_CBOLT", MisAcid: "MIS_ACID", MisAcidPud: "MIS_ACIDPUD",
	MisApoca: "MIS_APOCA", MisArrow: "MIS_ARROW", MisFarrow: "MIS_FARROW", MisNova: "MIS_NOVA", MisFireRing: "MIS_FIRERING",
	MisFireWall: "MIS_FIREWALL", MisFlashTrap: "MIS_FLASHTRAP"}

// info is the mod's missiles.rec record for mis.
func (c *missileCtx) info(mis MonsterMissile) rules.MissileData {
	return c.ctx.GameState.factory.Gen().D.Missiles[missileIDs[mis]]
}

// secPerTile is the flight speed from the missile's velocity.
func (c *missileCtx) secPerTile(mis MonsterMissile) float64 {
	return rules.MissileSecondsPerTile(max(c.info(mis).Velocity, 1))
}

// MonsterMissileAction resolves one monster missile at once on the grid, like the player's spells (decision 6):
// bolts fly until they hit the player or a wall, lines and flashes roll D1's per-tick hits in one go.
type MonsterMissileAction struct {
	BaseAction
	caster *Actor
	mis    MonsterMissile
	target geometry.Point
	dam    int
}

func NewMonsterMissileAction(caster *Actor, mis MonsterMissile, target geometry.Point, dam int) *MonsterMissileAction {
	return &MonsterMissileAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameMonsterMissile, caster), name: ActionNameMonsterMissile},
		caster: caster, mis: mis, target: target, dam: dam}
}

func (a *MonsterMissileAction) Execute(ctx ActionContext) []Action {
	m := ctx.GameState.GetMap()
	t, _ := m.TryGetActorAt(a.target)
	c := &missileCtx{ctx: ctx, caster: a.caster, mon: a.caster.monster, player: ctx.GameState.GetPlayer(),
		golem: t != nil && t.monster != nil && t.monster.Golem,
		c:     &spellCtx{f: ctx.GameState.factory, m: m, caster: a.caster, cause: a.cause, dealt64: map[*Actor]int{}}}
	c.fire(a.mis, a.target, a.dam)
	c.c.emit(ctx.ActionBroker)
	hits := c.c.finish() // the golem, if it was in the way
	if c.dealt <= 0 || !c.player.IsAlive() {
		return hits
	}
	return append(hits, NewChangeResourceAction(a.cause, c.player, stats.Health, -float64(c.dealt)))
}

type missileCtx struct {
	ctx    ActionContext
	caster *Actor         // nil for a trap
	mon    *Monster       // nil for a trap
	from   geometry.Point // a trap's cell
	player *Actor
	c      *spellCtx // for its grid helpers
	dealt  int       // whole points
	taken  int       // 64ths that hit the hero, before mana shield
	block  bool      // the hero blocked a hit
	trap   bool      // sourceless: it hits monsters too (MonsterTrapHit)
	golem  bool      // a monster's missile at the golem (MFLAG_TARGETS_MONSTER): it hits the golem too
}

// hit ports PlayerMHit: `ticks` hit rolls against the player standing at p, from a missile that flew from `from`.
func (c *missileCtx) hit(mis MonsterMissile, p, from geometry.Point, mind, maxd int, shift bool, ticks int) bool {
	return c.hitAs(mis, p, from, mind, maxd, shift, ticks, c.info(mis).Resist)
}

// hitAs is hit with the missile's resistance kind given: a fire arrow's flight hits as a plain arrow (MI_LArrow).
func (c *missileCtx) hitAs(mis MonsterMissile, p, from geometry.Point, mind, maxd int, shift bool, ticks, resist int) bool {
	c.ctx.GameState.factory.hitObject(c.c.m, p, c.attacker())
	if (c.trap || c.golem) && c.trapHitsMonster(p, from, mind, maxd, shift, ticks, resist) {
		return true
	}
	pl := c.player
	if pl.hero == nil || !pl.IsAlive() || pl.MapPosition() != p {
		return false
	}
	h, s := pl.hero, pl.HeroSheet()
	dist, dlvl := geometry.DistanceChebyshev(from, p), c.ctx.GameState.CurrentLevel().Depth
	monLevel := h.Level // PlayerMHit without a monster: no level difference
	var chance int
	if c.mon != nil {
		monLevel = c.mon.Level
		chance = h.MonsterSpellHitChance(c.mon.Level, dlvl)
	} else {
		chance = rules.TrapHitChance(s, mis != MisArrow && mis != MisFarrow, dist, dlvl) // missiledata's mType
	}
	resper := map[int]int{rules.ResistKindFire: s.FR, rules.ResistKindLightning: s.LR, rules.ResistKindMagic: s.MR}[resist]
	block := 0
	if !shift && mis != MisAcidPud {
		block = h.BlockChance(s, monLevel)
	}
	total, blocked, flinch := 0, false, 0
	for i := 0; i < ticks; i++ {
		if rng.Intn(100) >= chance {
			continue
		}
		dam := rules.MissileDamage64(rng, s, mind, maxd, shift)
		if c.mon == nil {
			dam = rules.TrapDamage64(rng, s, mind, maxd, shift)
		}
		if resper > 0 { // a resisted hit doesn't flinch
			dam -= resper * dam / 100
		} else if rng.Intn(100) < block {
			blocked = true
			continue
		} else {
			flinch = max(flinch, dam)
		}
		total += dam
	}
	if blocked {
		blockWear(c.ctx.ActionBroker, c.ctx.Msg, pl)
	}
	if blocked && total == 0 {
		c.block = true
		playSfxAt(c.ctx, "IS_ISWORD", p) // StartPlrBlock
		if c.caster != nil {
			c.ctx.Msg(fmt.Sprintf("You block the attack of %s.", c.caster.Name()))
		}
	}
	if total == 0 {
		return false
	}
	c.taken += total
	c.dealt += manaShield(pl, rules.Whole(total))
	if flinch > 0 {
		StartPlrHit(c.ctx.GameState.factory, pl, flinch, c.dealt, false)
	}
	c.c.flash(p, 0.3, defs.IconEffectHit)
	return true
}

// trapHitsMonster ports CheckMissileCol's sourceless half, MonsterTrapHit: a trap's missile hits the monster in its
// way too, 90 - AC - distance to hit, a quarter through a resistance.
func (c *missileCtx) trapHitsMonster(p, from geometry.Point, mind, maxd int, shift bool, ticks, resist int) bool {
	a, ok := c.c.m.TryGetActorAt(p)
	if !ok || a.monster == nil || rules.Fixed(a.GetStats().GetResource(stats.Health).Value()) <= c.c.dealt64[a] {
		return false
	}
	mon := a.monster
	if !c.trap && !mon.Golem {
		return false
	}
	res, immune := resistBits(resist)
	if mon.TalkMsg != "" || mon.MagicRes&immune != 0 || untouchable(c.c.f.Generator.D, mon, true) {
		return false
	}
	hper := max(min(90-mon.AC-geometry.DistanceChebyshev(from, p), 95), 5)
	total := 0
	for i := 0; i < ticks; i++ {
		if rng.Intn(100) < hper || a.HasStatusEffect(StatusStone) {
			dam := mind + rng.Intn(max(maxd-mind+1, 1))
			if !shift {
				dam <<= 6
			}
			if mon.MagicRes&res != 0 {
				dam >>= 2
			}
			total += dam
		}
	}
	if total == 0 {
		return false
	}
	if _, seen := c.c.dealt64[a]; !seen {
		c.c.order = append(c.c.order, a)
	}
	c.c.dealt64[a] += total
	c.c.flash(p, 0.3, defs.IconEffectHit)
	return true
}

// origin is where the missile starts: the caster, or the trap.
func (c *missileCtx) origin() geometry.Point {
	if c.caster != nil {
		return c.caster.MapPosition()
	}
	return c.from
}

// attacker is who breaks the barrels a missile hits: the caster, or the hero for a trap's missile.
func (c *missileCtx) attacker() *Actor {
	if c.caster != nil {
		return c.caster
	}
	return c.player
}

func (c *missileCtx) monsterDamage() int {
	return c.mon.MinDam + rng.Intn(max(c.mon.MaxDam-c.mon.MinDam+1, 1))
}

func (c *missileCtx) icon(mis MonsterMissile) int32 {
	switch c.info(mis).Resist {
	case rules.ResistKindFire:
		return defs.IconEffectFireSmall
	case rules.ResistKindLightning:
		return defs.IconEffectLightning
	}
	return defs.IconEffectHit
}

// bolt flies towards the target until it hits the player or something solid (MI_Firebolt, CheckMissileCol).
func (c *missileCtx) bolt(mis MonsterMissile, to geometry.Point, mind, maxd int) (geometry.Point, bool) {
	return c.fly(mis, c.c.ray(c.origin(), to, 20), mind, maxd, false)
}

// fly moves a missile through cells until it hits; a wandering one is drawn through every cell, not straight.
func (c *missileCtx) fly(mis MonsterMissile, cells []geometry.Point, mind, maxd int, wander bool) (geometry.Point, bool) {
	from, hit := c.origin(), false
	last, path := from, []geometry.Point{from}
	for _, p := range cells {
		if o := objectAt(c.c.m, p); o != nil && !o.IsWalkable(nil) { // monster missiles fly through other monsters
			c.ctx.GameState.factory.hitObject(c.c.m, p, c.attacker())
			break
		}
		last = p
		path = append(path, p)
		if hit = c.hit(mis, p, from, mind, maxd, false, 1); hit {
			break
		}
	}
	if impact := c.info(mis).ImpactSfx; last != from {
		flight := []geometry.Point{from, last}
		if wander {
			flight = path
		}
		c.c.events = append(c.c.events, Missile{By: c.caster, Path: flight, SecPerTile: c.secPerTile(mis), Icon: c.icon(mis), ArriveSfx: impact})
	} else {
		playSfxAt(c.ctx, impact, last)
	}
	return last, hit
}

func (c *missileCtx) fire(mis MonsterMissile, target geometry.Point, dam int) {
	o := c.origin()
	playSfxAt(c.ctx, c.info(mis).LaunchSfx, o)
	switch mis {
	case MisFirebolt, MisMagmaball, MisFlare:
		d := c.monsterDamage()
		c.bolt(mis, target, d, d)
	case MisAcid:
		d := c.monsterDamage()
		if end, _ := c.bolt(mis, target, d, d); end != o { // MI_Acidsplat
			d := 1
			if c.mon.Data.Level >= 2 {
				d = 2
			}
			c.hazard(MisAcidPud, end, (rng.Intn(15)+40*(c.mon.AILevel+1))/rules.RefWalkTicks, d)
		}
	case MisCbolt:
		for i := 0; i < 3; i++ { // M_DoRAttack: three sparks, each wandering like the hero's (MI_Cbolt)
			c.fly(mis, c.c.chargedBoltCells(o, target, rng.Intn(15)+1), 15, 15, true)
		}
	case MisFireball:
		end, _ := c.bolt(mis, target, dam, dam)
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
			c.c.missile(c.caster, []geometry.Point{o, cells[len(cells)-1]}, c.secPerTile(mis), c.icon(mis))
		}
	case MisLightning:
		c.hit(mis, target, target, dam, dam, true, 8)
		playSfxAt(c.ctx, c.info(mis).ImpactSfx, target)
		c.c.flash(target, 0.3, defs.IconEffectLightning)
	case MisFlash: // the six cells from one left of the caster to one right and below
		for _, d := range []geometry.Point{{X: -1}, {}, {X: 1}, {X: -1, Y: 1}, {Y: 1}, {X: 1, Y: 1}} {
			c.hit(mis, o.Add(d), o, 2*c.mon.Level, 2*c.mon.Level, true, rules.FlashTicks)
		}
	case MisApoca: // AddDiabApoca: MIS_BOOM2 on the player when the line is clear
		if p := c.player.MapPosition(); c.c.lineClear(o, p) {
			c.hit(mis, p, o, dam, dam, false, 1)
			c.c.flash(p, 0.4, defs.IconEffectFireBig)
		}
	case MisFlamec: // MI_Flamec: flames on the next three cells towards the target, the n-th burning 20+5n ticks
		for n, p := range c.c.ray(o, target, 3) {
			c.hazard(MisFlamec, p, (20+5*n)/rules.RefWalkTicks, c.monsterDamage())
		}
	}
}

// hazard leaves a burning flame or acid puddle that hits the player on it every turn (8 ticks, 1/64 damage each).
func (c *missileCtx) hazard(mis MonsterMissile, p geometry.Point, turns, dam64 int) {
	f := c.ctx.GameState.factory
	m := c.c.m
	if turns <= 0 || m.IsObjectAt(p) || !m.IsWalkable(p) {
		return
	}
	caster, mon, trap := c.caster, c.mon, c.trap
	hz := NewFire(0)
	hz.turnsUntilRemoval, hz.FireWall = turns, mis == MisFireRing || mis == MisFireWall
	if mis == MisAcidPud { // the other hazards' fire() already played their launch
		hz.icon = defs.IconEffectHit
		playSfxAt(c.ctx, c.info(mis).LaunchSfx, p)
	}
	burn := func() {
		// a trap's fire burns whoever stands in it; a monster's only the hero
		if f.State.GetMap() != m || c.player.MapPosition() != p && !(trap && m.IsActorAt(p)) {
			return
		}
		f.ActionBroker.Execute(&hazardTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			mc: &missileCtx{caster: caster, mon: mon, trap: trap}, mis: mis, p: p, dam64: dam64})
	}
	hz.SetDealDamage(func(geometry.Point) {})
	hz.SetOnTurn(func() {
		if !hz.IsAlive() || f.State.GetMap() != m {
			m.RemoveObject(hz)
			hz.turnsUntilRemoval = 0
			return
		}
		burn()
	})
	f.GetTimeKeeper().AddTimed(hz)
	m.AddObject(hz, p)
	burn()
}

type hazardTickAction struct {
	BaseAction
	mc    *missileCtx
	mis   MonsterMissile
	p     geometry.Point
	dam64 int
}

func (a *hazardTickAction) Execute(ctx ActionContext) []Action {
	c := a.mc
	c.ctx, c.player, c.dealt = ctx, ctx.GameState.GetPlayer(), 0
	c.c = &spellCtx{f: ctx.GameState.factory, m: ctx.GameState.GetMap(), caster: c.caster, cause: a.cause, dealt64: map[*Actor]int{}}
	c.hit(a.mis, a.p, a.p, a.dam64, a.dam64, true, rules.RefWalkTicks)
	c.c.emit(ctx.ActionBroker)
	hits := c.c.finish()
	if c.dealt <= 0 {
		return hits
	}
	return append(hits, NewChangeResourceAction(a.cause, c.player, stats.Health, -float64(c.dealt)))
}

// D1AcidDeath: acid beasts leave a puddle where they die (M_StartKill).
func (f *Factory) acidDeath(ctx ActionContext, victim *Actor) {
	if mon := victim.monster; mon.Data.Has("acid_blood") {
		c := &missileCtx{ctx: ctx, caster: victim, mon: mon, player: ctx.GameState.GetPlayer(), c: &spellCtx{m: ctx.GameState.GetMap(), caster: victim}}
		p := victim.MapPosition()
		c.hazard(MisAcidPud, p, (rng.Intn(15)+40*(mon.AILevel+1))/rules.RefWalkTicks, mon.AILevel+1)
	}
}
