package common

import (
	"fmt"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
)

// D1 spell status effects (decision 6: Mana Shield, Infravision and Stone Curse are status effects).
const (
	StatusStone       StatusEffect = "stone"
	StatusManaShield  StatusEffect = "mana_shield"
	StatusInfravision StatusEffect = "infravision"
)

var d1SpellIcons = map[string]int32{
	"STYPE_FIRE": defs.IconEffectFireSmall, "STYPE_LIGHTNING": defs.IconEffectLightning, "STYPE_MAGIC": defs.IconEffectHit,
}

// SpellAction is one cast of a D1 spell, resolved instantly (decision 6).
type SpellAction struct {
	BaseAction
	caster        *Actor
	spell, slvl   int
	target        geometry.Point
	animations    []Animation
	castSucceeded bool
}

func NewSpellAction(caster *Actor, spell, slvl int, target geometry.Point) *SpellAction {
	return &SpellAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
		caster: caster, spell: spell, slvl: slvl, target: target}
}

func (a *SpellAction) GetAnimations() []Animation { return a.animations }
func (a *SpellAction) GetDelayInAut() int         { return D1CastAut(a.caster) }

// Succeeded: false when the spell found nothing to act on (D1 then doesn't take mana).
func (a *SpellAction) Succeeded() bool { return a.castSucceeded }

func (a *SpellAction) Execute(ctx ActionContext) []Action {
	f := ctx.GameState.factory
	c := &spellCtx{f: f, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), spell: a.spell, slvl: a.slvl,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt: map[*Actor]int{}, ctx: ctx}
	a.castSucceeded = c.cast(a.target)
	if a.castSucceeded {
		ctx.Msg(fmt.Sprintf("You cast %s.", f.d1.D.Spells[a.spell].Name))
	}
	a.animations = c.animations
	return c.finish()
}

// D1CastAut: casting takes the class's spell animation, rounded to a speed tier (decision 13).
func D1CastAut(caster *Actor) int {
	class := 0
	if caster.hero != nil {
		class = caster.hero.Class
	}
	speed := float64(caster.GetBaseSpeed()) * d1.SpeedTier(d1.RefAttackTicks, d1.SpellFrames[class])
	return 100 / max(int(speed), 1)
}

type spellCtx struct {
	f           *Factory
	ctx         ActionContext
	caster      *Actor
	h           *d1.Hero
	s           d1.Sheet
	spell, slvl int
	m           *gridmap.GridMap[*Actor, *Item, Object]
	cause       Cause
	dealt       map[*Actor]int // whole points, applied once per victim in finish
	order       []*Actor
	animations  []Animation
}

func (c *spellCtx) origin() geometry.Point { return c.caster.MapPosition() }

func (c *spellCtx) monsterAt(p geometry.Point) *Actor {
	if a, ok := c.m.TryGetActorAt(p); ok && a.d1Monster != nil && !a.d1Monster.Golem && a.GetStats().GetResource(stats.Health).Value() > c.dealt[a] {
		return a
	}
	return nil
}

func (c *spellCtx) open(p geometry.Point) bool { return c.m.Contains(p) && c.m.IsTransparent(p) }

// ray lists the cells from `from` (excluded) through `to` and on, up to maxLen cells or the first wall.
func (c *spellCtx) ray(from, to geometry.Point, maxLen int) []geometry.Point {
	d := to.Sub(from)
	n := max(geometry.Abs(d.X), geometry.Abs(d.Y))
	if n == 0 {
		return nil
	}
	k := (maxLen + n - 1) / n
	far := from.Add(geometry.Point{X: d.X * k, Y: d.Y * k})
	var cells []geometry.Point
	for _, p := range geometry.LineOfSight(from, far, func(p geometry.Point) bool { return p == from || c.open(p) })[1:] {
		if !c.open(p) || len(cells) == maxLen {
			break
		}
		cells = append(cells, p)
	}
	return cells
}

// lineClear ports LineClear/CheckBlock: no wall between the two cells.
func (c *spellCtx) lineClear(from, to geometry.Point) bool {
	los := geometry.LineOfSight(from, to, func(p geometry.Point) bool { return p == from || p == to || c.open(p) })
	return los[len(los)-1] == to
}

// freeNear ports the CrawlTable searches: the nearest cell (ring by ring, up to radius 5) that is floor, empty
// and in line from the caster.
func (c *spellCtx) freeNear(target geometry.Point, ok func(geometry.Point) bool) (geometry.Point, bool) {
	for r := 0; r <= 5; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				p := target.Add(geometry.Point{X: dx, Y: dy})
				if max(geometry.Abs(dx), geometry.Abs(dy)) == r && c.m.Contains(p) && ok(p) {
					return p, true
				}
			}
		}
	}
	return geometry.Point{}, false
}

func (c *spellCtx) emptyFloor(p geometry.Point) bool {
	return c.m.IsCurrentlyPassable(p) && !c.m.IsObjectAt(p) && c.lineClear(c.origin(), p)
}

func resistBits(kind int) (resist, immune int) {
	switch kind {
	case d1.ResistKindFire:
		return d1.ResistFire, d1.ImmuneFire
	case d1.ResistKindLightning:
		return d1.ResistLightning, d1.ImmuneLightning
	case d1.ResistKindMagic:
		return d1.ResistMagic, d1.ImmuneMagic
	}
	return 0, 0
}

// strike ports MonsterMHit for a player's spell: `ticks` hit rolls of dam64 (1/64) each.
func (c *spellCtx) strike(a *Actor, dam64, ticks int) bool {
	mon := a.d1Monster
	resist, immune := resistBits(d1.SpellResist[c.spell])
	if mon.MagicRes&immune != 0 || c.spell == d1.SplHolyBolt && mon.Data.Class != "MC_UNDEAD" && mon.AI != "AI_DIABLO" {
		return false
	}
	if c.spell == d1.SplBoneSpirit {
		dam64 = (a.GetStats().GetResource(stats.Health).Value() - c.dealt[a]) << 6 / 3
	}
	dist := geometry.DistanceChebyshev(c.origin(), a.MapPosition())
	chance := c.h.SpellHitChance(c.s, mon.Level, dist)
	total := 0
	for i := 0; i < ticks; i++ {
		if a.HasStatusEffect(StatusStone) || d1Rng.Intn(100) < chance {
			total += dam64
		}
	}
	if total == 0 {
		return false
	}
	if mon.MagicRes&resist != 0 {
		total >>= 2
	}
	d1Alert(a, c.caster)
	d1MonsterGotHit(a, total>>6)
	if _, seen := c.dealt[a]; !seen {
		c.order = append(c.order, a)
	}
	c.dealt[a] += total >> 6
	c.animations = append(c.animations, NewHitAnimation(a.MapPosition(), 0.3, defs.IconEffectHit, 2))
	return true
}

func (c *spellCtx) finish() []Action {
	var actions []Action
	for _, a := range c.order {
		if c.dealt[a] > 0 {
			actions = append(actions, NewChangeResourceAction(c.cause, a, stats.Health, -float64(c.dealt[a])))
		}
	}
	return actions
}

func (c *spellCtx) icon() int32 { return d1SpellIcons[c.f.d1.D.Spells[c.spell].Type] }

// bolt flies from `from` towards `to` until it hits a monster or a wall; a miss flies on (CheckMissileCol).
// It returns the last cell it reached. Barrels in its way break.
func (c *spellCtx) bolt(from, to geometry.Point, hit func(a *Actor) bool) geometry.Point {
	last := from
	for _, p := range c.ray(from, to, 20) {
		last = p
		if o := d1ObjectAt(c.m, p); o != nil && !o.IsWalkable(nil) {
			if o.O.Kind == d1.ObjBarrel || o.O.Kind == d1.ObjBarrelEx {
				c.f.breakD1Barrel(o, c.caster)
			}
			break
		}
		if a := c.monsterAt(p); a != nil && hit(a) {
			break
		}
	}
	if last != from {
		c.animations = append(c.animations, NewProjectileDirectionAnimation(c.caster, from, last, 0.03, c.icon(), 2))
	}
	return last
}

// beam hits every monster on the line up to the first wall (lightning).
func (c *spellCtx) beam(to geometry.Point, dam func() int) {
	cells := c.ray(c.origin(), to, 20)
	for _, p := range cells {
		if a := c.monsterAt(p); a != nil {
			c.strike(a, dam()<<6, 1)
		}
	}
	if len(cells) > 0 {
		c.animations = append(c.animations, NewProjectileDirectionAnimation(c.caster, c.origin(), cells[len(cells)-1], 0.01, c.icon(), 2))
	}
}

// home ports the second half of Elemental and Bone Spirit: from where they arrived, find the closest monster.
func (c *spellCtx) home(from geometry.Point, dam64 int) {
	var best *Actor
	for _, a := range c.m.Actors() {
		if c.monsterAt(a.MapPosition()) == a && geometry.DistanceChebyshev(from, a.MapPosition()) < 19 && c.lineClear(from, a.MapPosition()) &&
			(best == nil || geometry.DistanceSquared(from, a.MapPosition()) < geometry.DistanceSquared(from, best.MapPosition())) {
			best = a
		}
	}
	if best != nil && best.MapPosition() != from {
		c.bolt(from, best.MapPosition(), func(a *Actor) bool { return c.strike(a, dam64, 1) })
	}
}

func (c *spellCtx) payHP(hp int) {
	c.caster.GetStats().GetResource(stats.Health).Decrease(hp)
	if !c.caster.IsAlive() {
		c.f.actionBroker.QueueForExecution(NewDieAction(c.cause, c.caster))
	}
}

func (c *spellCtx) cast(target geometry.Point) bool {
	f, h, s, slvl, o := c.f, c.h, c.s, c.slvl, c.origin()
	clvl := h.Level
	st := c.caster.GetStats()
	inTown := defs.IsTownLevel(c.m.GetName())
	if inTown && !f.d1.D.Spells[c.spell].Town {
		c.ctx.Msg("That spell doesn't work in town.")
		return false
	}
	if d1.SpellTargeted[c.spell] && target == o && c.spell != d1.SplTeleport && c.spell != d1.SplGolem {
		c.ctx.Msg("You need a target other than yourself.")
		return false
	}
	switch c.spell {
	case d1.SplFirebolt, d1.SplHolyBolt, d1.SplBloodStar:
		dam := d1.FireboltDamage(d1Rng, s.Mag, slvl)
		switch c.spell {
		case d1.SplHolyBolt:
			dam = d1.HolyBoltDamage(d1Rng, clvl)
		case d1.SplBloodStar:
			dam = d1.BloodStarDamage(s.Mag, slvl)
			c.payHP(5)
		}
		c.bolt(o, target, func(a *Actor) bool { return c.strike(a, dam<<6, 1) })
	case d1.SplHeal:
		st.GetResource(stats.Health).Increase(float64(d1.HealAmount64(d1Rng, h.Class, clvl, slvl) >> 6))
	case d1.SplLightning:
		c.beam(target, func() int { return d1.LightningDamage(d1Rng, clvl) })
	case d1.SplChain:
		c.beam(target, func() int { return d1.LightningDamage(d1Rng, clvl) })
		for _, a := range append([]*Actor(nil), c.m.Actors()...) {
			if c.monsterAt(a.MapPosition()) == a && geometry.DistanceChebyshev(o, a.MapPosition()) < min(slvl+3, 19) {
				c.beam(a.MapPosition(), func() int { return d1.LightningDamage(d1Rng, clvl) })
			}
		}
	case d1.SplFlash:
		low, high := d1.FlashDamage64(d1Rng, clvl, slvl)
		for dx := -1; dx <= 1; dx++ {
			for dy, dam := range map[int]int{-1: high, 0: low, 1: low} {
				if a := c.monsterAt(o.Add(geometry.Point{X: dx, Y: dy})); a != nil {
					c.strike(a, dam, d1.FlashTicks)
				}
			}
		}
		c.animations = append(c.animations, NewHitAnimation(o, 0.4, defs.IconEffectLightning, 2))
	case d1.SplIdentify:
		return f.openD1IdentifyMenu(c.caster)
	case d1.SplFireWall:
		start, ok := c.freeNear(target, func(p geometry.Point) bool {
			return p != o && c.m.IsTileWalkable(p) && !c.m.IsObjectAt(p) && c.lineClear(o, p)
		})
		if !ok {
			return false
		}
		dir := D1DirTo(o, target)
		f.spawnD1Fire(c.caster, start, slvl)
		for _, side := range []int{dir + 2, dir + 6} {
			p := start
			for i := 1; i < 7; i++ {
				p = p.Add(D1Dirs[side%8])
				if !c.m.Contains(p) || !c.m.IsTileWalkable(p) {
					break
				}
				f.spawnD1Fire(c.caster, p, slvl)
			}
		}
	case d1.SplTown:
		return f.openD1Portal(c.caster)
	case d1.SplStone:
		var victim *Actor
		c.freeNear(target, func(p geometry.Point) bool {
			victim = c.monsterAt(p)
			return victim != nil && victim.d1Monster.AI != "AI_DIABLO"
		})
		if victim == nil {
			return false
		}
		victim.AddStatusEffect(StatusStone, d1.StoneTurns(slvl))
		c.animations = append(c.animations, NewHitAnimation(victim.MapPosition(), 0.4, defs.IconEffectHit, 2))
	case d1.SplInfra:
		c.caster.AddStatusEffect(StatusInfravision, d1.InfravisionTurns(slvl))
	case d1.SplPhasing:
		f.d1RandomTeleport(c.caster)
	case d1.SplManaShield:
		c.caster.AddStatusEffect(StatusManaShield, -1)
	case d1.SplFireball:
		dam := d1.FireballDamage(d1Rng, clvl, slvl)
		impact := c.bolt(o, target, func(a *Actor) bool { return c.strike(a, dam<<6, 1) })
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				p := impact.Add(geometry.Point{X: dx, Y: dy})
				if a := c.monsterAt(p); a != nil && c.lineClear(o, p) {
					c.strike(a, dam<<6, 1)
				}
			}
		}
		c.animations = append(c.animations, NewHitAnimation(impact, 0.4, defs.IconEffectFireBig, 2))
	case d1.SplGuardian:
		p, ok := c.freeNear(target, c.emptyFloor)
		if !ok {
			return false
		}
		f.spawnD1Guardian(c.caster, p, d1.GuardianTurns(clvl, slvl))
	case d1.SplWave:
		dir := D1DirTo(o, target)
		start := o.Add(D1Dirs[dir])
		if !c.open(start) {
			return true
		}
		starts := []geometry.Point{start}
		for _, side := range []int{dir + 6, dir + 2} {
			p := start
			for j := 0; j < slvl>>1+2; j++ {
				p = p.Add(D1Dirs[side%8])
				if !c.open(p) {
					break
				}
				starts = append(starts, p)
			}
		}
		for _, p := range starts {
			dam := d1.WaveDamage(d1Rng, clvl)
			cells := append([]geometry.Point{p}, c.ray(p, p.Add(D1Dirs[dir]), 20)...)
			for _, q := range cells {
				if a := c.monsterAt(q); a != nil {
					c.strike(a, dam<<6, 1)
				}
			}
			c.animations = append(c.animations, NewProjectileDirectionAnimation(c.caster, p, cells[len(cells)-1], 0.03, defs.IconEffectFireSmall, 2))
		}
	case d1.SplNova:
		dam := d1.NovaDamage(d1Rng, clvl, slvl)
		for _, r := range d1.NovaRays {
			for _, p := range c.ray(o, o.Add(geometry.Point{X: r[0], Y: r[1]}), 20) {
				if a := c.monsterAt(p); a != nil {
					c.strike(a, dam<<6, 1)
				}
			}
		}
		c.animations = append(c.animations, NewHitAnimation(o, 0.5, defs.IconEffectLightning, 2))
	case d1.SplInferno:
		for n, p := range c.ray(o, target, 3) {
			dam := d1.InfernoDamage64(d1Rng, clvl)
			if a := c.monsterAt(p); a != nil {
				c.strike(a, dam, d1.InfernoTicks(n))
			}
			c.animations = append(c.animations, NewHitAnimation(p, 0.5, defs.IconEffectFireBig, 2))
		}
	case d1.SplGolem:
		return f.summonD1Golem(c, target)
	case d1.SplTeleport:
		p, ok := c.freeNear(target, func(p geometry.Point) bool { return c.m.IsCurrentlyPassable(p) && !c.m.IsObjectAt(p) })
		if !ok {
			return false
		}
		f.d1MovePlayer(c.caster, p, "Teleport")
	case d1.SplApocalypse:
		dam := d1.ApocalypseDamage(d1Rng, clvl)
		for _, a := range append([]*Actor(nil), c.m.Actors()...) {
			if d := a.MapPosition().Sub(o); c.monsterAt(a.MapPosition()) == a && d.X >= -8 && d.X < 8 && d.Y >= -8 && d.Y < 8 {
				c.strike(a, dam<<6, 1)
			}
		}
	case d1.SplElemental, d1.SplBoneSpirit:
		dam := d1.ElementalDamage(d1Rng, clvl, slvl)
		if c.spell == d1.SplBoneSpirit {
			c.payHP(6)
		}
		hit := false
		arrived := c.bolt(o, target, func(a *Actor) bool {
			hit = c.strike(a, dam<<6, 1)
			return hit
		})
		if !hit && geometry.DistanceChebyshev(o, arrived) >= geometry.DistanceChebyshev(o, target) {
			c.home(arrived, dam<<6)
		}
	case d1.SplChargedBolt:
		bpath := [16]int{-1, 0, 1, -1, 0, 1, -1, -1, 0, 0, 1, 1, 0, 1, -1, 0}
		for n := d1.ChargedBolts(slvl); n > 0; n-- {
			dam, dir, r := d1.ChargedBoltDamage(d1Rng, s.Mag), D1DirTo(o, target), d1Rng.Intn(15)+1
			p, md := o, dir
			for step := 0; step < 24; step++ {
				if step%4 == 0 {
					md = ((dir+bpath[r])%8 + 8) % 8
					r = (r + 1) & 15
				}
				next := p.Add(D1Dirs[md])
				if !c.open(next) {
					break
				}
				p = next
				if a := c.monsterAt(p); a != nil && c.strike(a, dam<<6, 1) {
					break
				}
			}
			if p != o {
				c.animations = append(c.animations, NewProjectileDirectionAnimation(c.caster, o, p, 0.03, defs.IconEffectLightning, 2))
			}
		}
	case d1.SplTelekinesis:
		return f.d1Telekinesis(c, target)
	default:
		return false
	}
	return true
}

// spawnD1Fire places one fire wall cell: each turn it burns whoever stands in it (MI_Firewall, 8 ticks a turn).
func (f *Factory) spawnD1Fire(caster *Actor, p geometry.Point, slvl int) {
	m := f.state.GetMap()
	if m.IsObjectAt(p) {
		return
	}
	fire := NewFire(0)
	fire.turnsUntilRemoval = d1.FireWallTurns(slvl)
	fire.SetDealDamage(func(location geometry.Point, _ func() bool) {})
	fire.SetOnTurn(func() {
		if !fire.IsAlive() || f.state.GetMap() != m {
			m.RemoveObject(fire)
			fire.turnsUntilRemoval = 0
			return
		}
		if a, ok := m.TryGetActorAt(p); ok && a.d1Monster != nil && caster.IsAlive() {
			f.actionBroker.Execute(&spellTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
				caster: caster, spell: d1.SplFireWall, run: func(c *spellCtx) {
					if v := c.monsterAt(p); v != nil {
						c.strike(v, d1.FireWallDamage64(d1Rng, caster.hero.Level), d1.RefWalkTicks)
					}
				}})
		}
	})
	f.getTimeKeeper().AddTimed(fire)
	m.AddObject(fire, p)
}

// spawnD1Guardian places a Guardian: every other turn it spits a firebolt at the closest monster it can see (MI_Guardian).
func (f *Factory) spawnD1Guardian(caster *Actor, p geometry.Point, turns int) {
	m := f.state.GetMap()
	g := NewFire(0)
	g.icon, g.frameCount, g.turnsUntilRemoval = defs.IconEffectFireBig, 2, turns
	g.SetDealDamage(func(geometry.Point, func() bool) {})
	g.SetOnTurn(func() {
		if !g.IsAlive() || f.state.GetMap() != m {
			m.RemoveObject(g)
			g.turnsUntilRemoval = 0
			return
		}
		if g.turnsUntilRemoval%2 != 0 || caster.hero == nil {
			return
		}
		f.actionBroker.Execute(&spellTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			caster: caster, spell: d1.SplFirebolt, run: func(c *spellCtx) {
				var best *Actor
				for _, a := range m.Actors() {
					if c.monsterAt(a.MapPosition()) == a && geometry.DistanceChebyshev(p, a.MapPosition()) <= 15 && c.lineClear(p, a.MapPosition()) &&
						(best == nil || geometry.DistanceSquared(p, a.MapPosition()) < geometry.DistanceSquared(p, best.MapPosition())) {
						best = a
					}
				}
				if best != nil {
					dam := d1.FireboltDamage(d1Rng, c.s.Mag, c.h.SpellLevel(c.s, d1.SplFirebolt))
					c.bolt(p, best.MapPosition(), func(a *Actor) bool { return c.strike(a, dam<<6, 1) })
				}
			}})
	})
	f.getTimeKeeper().AddTimed(g)
	m.AddObject(g, p)
}

// spellTickAction resolves the damage of a lingering spell (fire wall, guardian) on its turn.
type spellTickAction struct {
	BaseAction
	caster     *Actor
	spell      int
	run        func(c *spellCtx)
	animations []Animation
}

func (a *spellTickAction) GetAnimations() []Animation { return a.animations }
func (a *spellTickAction) GetDelayInAut() int         { return 0 }
func (a *spellTickAction) Execute(ctx ActionContext) []Action {
	c := &spellCtx{f: ctx.GameState.factory, ctx: ctx, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), spell: a.spell,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt: map[*Actor]int{}}
	a.run(c)
	a.animations = c.animations
	return c.finish()
}

// d1MovePlayer puts the player on a cell (Teleport, Phasing, the Holy shrine).
func (f *Factory) d1MovePlayer(player *Actor, p geometry.Point, why ActionName) {
	m := f.state.GetMap()
	m.MoveActor(player, p)
	player.SetDrawPosition(p.ToPointF())
	f.actionBroker.QueueForExecution(NewUpdateFoVAction(NewActorCause(why, player), player))
}

// d1RandomTeleport ports MIS_RNDTELEPORT: a jump of 4-6 cells along each axis.
func (f *Factory) d1RandomTeleport(player *Actor) {
	m, p := f.state.GetMap(), player.MapPosition()
	for try := 0; try < 500; try++ {
		dx, dy := d1Rng.Intn(3)+4, d1Rng.Intn(3)+4
		if d1Rng.Intn(2) == 1 {
			dx = -dx
		}
		if d1Rng.Intn(2) == 1 {
			dy = -dy
		}
		if q := p.Add(geometry.Point{X: dx, Y: dy}); m.CanPlaceActorHere(q) && !m.IsObjectAt(q) {
			f.d1MovePlayer(player, q, "Phasing")
			return
		}
	}
}

func (f *Factory) openD1IdentifyMenu(player *Actor) bool {
	var items []util.MenuItem
	for _, it := range player.GetInventory().Items() {
		if it.d1 == nil || it.d1.Magical == 0 || it.d1.Identified {
			continue
		}
		item := it
		items = append(items, util.MenuItem{MainText: item.Name(), Action: func() {
			item.d1.Identified = true
			item.RefreshD1()
			SyncHero(player)
			f.logError(fmt.Sprintf("It is %s.", item.Name()))
		}})
	}
	if len(items) == 0 {
		f.logError("You have nothing to identify.")
		return false
	}
	if f.gui != nil {
		f.gui().OpenMenu(items)
	}
	return true
}

// d1Telekinesis ports the telekinesis cursor: operate an object, fetch an item, or push a monster away.
func (f *Factory) d1Telekinesis(c *spellCtx, target geometry.Point) bool {
	m, player := c.m, c.caster
	if !player.CanSee(target) {
		return false
	}
	if o := d1ObjectAt(m, target); o != nil {
		f.operateD1Object(o, player)
		return true
	}
	if it, ok := m.TryGetItemAt(target); ok {
		m.RemoveItem(it)
		if it.d1 != nil && it.d1.Type == d1.ITypeGold {
			player.GetStats().IncrementBy(stats.Gold, it.d1.Value)
		} else {
			player.GetInventory().Add(it)
		}
		c.ctx.Msg(fmt.Sprintf("You pick up %s.", it.Name()))
		return true
	}
	if a := c.monsterAt(target); a != nil && !a.HasStatusEffect(StatusStone) {
		if p := target.Add(D1Dirs[D1DirTo(c.origin(), target)]); m.CanPlaceActorHere(p) {
			m.MoveActor(a, p)
			a.SetDrawPosition(p.ToPointF())
		}
		return true
	}
	return false
}

// summonD1Golem ports AddGolem/SpawnGolum: the old golem goes, a new one appears near the target.
func (f *Factory) summonD1Golem(c *spellCtx, target geometry.Point) bool {
	p, ok := c.freeNear(target, c.emptyFloor)
	if !ok {
		return false
	}
	for _, a := range append([]*Actor(nil), c.m.Actors()...) {
		if a.d1Monster != nil && a.d1Monster.Golem {
			c.m.RemoveActor(a)
		}
	}
	gs := d1.GolemStats(c.slvl, c.h.Level, c.s.MaxMana)
	a := f.NewD1Monster(f.d1.D.MonsterByID("MT_GOLEM"))
	a.isHostile = false
	mon := a.d1Monster
	mon.Golem, mon.SpawnIndex, mon.AC, mon.Hit, mon.MinDam, mon.MaxDam = true, -1, gs.AC, gs.Hit, gs.MinDam, gs.MaxDam
	a.SetColor(defs.White)
	a.GetStats().SetAttribute(stats.MaxHealth, gs.HP)
	a.GetStats().GetResource(stats.Health).SetToMax()
	a.SetDrawPosition(p.ToPointF())
	c.m.AddActor(a, p)
	f.state.OnActorSpawned(a)
	return true
}

// D1Portal is the open town portal: where it stands in the dungeon.
type D1Portal struct {
	Map  string
	X, Y int
}

const d1PortalLocation = "portal"

type d1PortalObject struct {
	pos    geometry.Point
	target gridmap.Transition
}

func (o *d1PortalObject) Name() string                        { return "Town Portal" }
func (o *d1PortalObject) MapPosition() geometry.Point         { return o.pos }
func (o *d1PortalObject) SetMapPosition(p geometry.Point)     { o.pos = p }
func (o *d1PortalObject) Icon(uint64) int32                   { return 72 }
func (o *d1PortalObject) IsWalkable(*Actor) bool              { return true }
func (o *d1PortalObject) IsTransparent() bool                 { return true }
func (o *d1PortalObject) IsPassableForProjectile() bool       { return true }
func (o *d1PortalObject) Turn()                               {}
func (o *d1PortalObject) ActorWalkedOn(*Actor, func() bool)   {}
func (o *d1PortalObject) ActorBumpedInto(*Actor, func() bool) {}

func (s *State) D1Portal() *D1Portal     { return s.d1Portal }
func (s *State) SetD1Portal(p *D1Portal) { s.d1Portal = p }

func (f *Factory) openD1Portal(player *Actor) bool {
	m := f.state.GetMap()
	if old := f.state.d1Portal; old != nil {
		f.state.CloseD1Portal()
	}
	cells := freeCellsNear(m, player.MapPosition(), 1, func(p geometry.Point) bool {
		_, isTransition := m.GetTransitionAt(p)
		return m.CanPlaceActorHere(p) && !m.IsObjectAt(p) && !isTransition
	})
	if len(cells) == 0 {
		return false
	}
	f.state.d1Portal = &D1Portal{Map: m.GetName(), X: cells[0].X, Y: cells[0].Y}
	PlaceD1Portal(f.state, m)
	return true
}

// PlaceD1Portal puts the town portal's two ends on a map when it has one: at the dungeon spot, or near the town spawn.
func PlaceD1Portal(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	p := s.d1Portal
	if p == nil {
		return
	}
	var pos geometry.Point
	var target gridmap.Transition
	switch {
	case m.GetName() == p.Map:
		// the town end is placed on arrival, after the transition looked up its target, so aim at the spawn next to it
		pos, target = geometry.Point{X: p.X, Y: p.Y}, gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"}
	case m.GetName() == string(defs.LevelNameTown):
		spawn := m.GetNamedLocation("spawn")
		cells := freeCellsNear(m, spawn, 3, func(q geometry.Point) bool {
			_, isTransition := m.GetTransitionAt(q)
			return m.CanPlaceActorHere(q) && !m.IsObjectAt(q) && !isTransition
		})
		if len(cells) < 3 {
			return
		}
		pos, target = cells[2], gridmap.Transition{TargetMap: p.Map, TargetLocation: d1PortalLocation}
	default:
		return
	}
	if m.IsObjectAt(pos) {
		return
	}
	m.AddNamedLocation(d1PortalLocation, pos)
	m.AddObject(&d1PortalObject{pos: pos, target: target}, pos)
	m.AddTransitionAt(pos, target)
}

// CloseD1Portal removes both ends of the town portal from the maps that are loaded.
func (s *State) CloseD1Portal() {
	if s.d1Portal == nil {
		return
	}
	maps := []*gridmap.GridMap[*Actor, *Item, Object]{s.GetDungeonMap(s.d1Portal.Map), s.GetTownMap(string(defs.LevelNameTown))}
	for _, m := range maps {
		if m == nil {
			continue
		}
		for _, obj := range append([]Object(nil), m.Objects()...) {
			if po, ok := obj.(*d1PortalObject); ok {
				m.RemoveObject(po)
				m.RemoveTransitionAt(po.pos)
			}
		}
	}
	s.d1Portal = nil
}

// D1SpellSkills lists the player's memorized spells as skills (the skill bar and the spell menu use them).
func (f *Factory) D1SpellSkills(player *Actor) ([]*TargetedAction, []*UntargetedAction) {
	h := player.hero
	if h == nil {
		return nil, nil
	}
	s := player.HeroSheet()
	var targeted []*TargetedAction
	var untargeted []*UntargetedAction
	for spell := range h.SpellLevels {
		if h.SpellLevel(s, spell) <= 0 {
			continue
		}
		sd := f.d1.D.Spells[spell]
		name := fmt.Sprintf("%s (lvl %d, %d mana)", sd.Name, h.SpellLevel(s, spell), f.d1.D.ManaCost(h, s, spell)>>6)
		sp := spell
		if d1.SpellTargeted[spell] {
			action := NewTargetedAction(name, OneTargetTile(f.state.GetMap, util.Arguments{}),
				&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { f.CastD1FromMemory(caster, sp, target) },
					Description: []util.LabelText{util.LabelTextFromString(sd.Name)}},
				d1SpellTargeting(f, sp))
			action.SetIcon(d1SpellIcons[sd.Type])
			targeted = append(targeted, action)
		} else {
			action := NewUntargetedAction(name, d1SpellIcons[sd.Type])
			action.SetEffect(&UntargetedEffect{Effect: func(caster *Actor, _ EffectSource) { f.CastD1FromMemory(caster, sp, caster.MapPosition()) },
				Description: []util.LabelText{util.LabelTextFromString(sd.Name)}})
			untargeted = append(untargeted, action)
		}
	}
	return targeted, untargeted
}

// RefreshD1Spells updates the player's spell skills after learning or changing gear.
func (f *Factory) RefreshD1Spells(player *Actor) {
	t, u := f.D1SpellSkills(player)
	player.SetTargetedSkills(t)
	player.SetUntargetedSkills(u)
}

func d1SpellTargeting(f *Factory, spell int) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, aoe *AoEResolver, m *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &RangedTargetingRules{
				attacker: attacker,
				isOnMap:  m.Contains,
				nearestHostileActor: func() geometry.Point {
					if a := m.GetNearestActor(attacker.MapPosition(), func(a *Actor) bool {
						return a.IsHostileTo(attacker) && attacker.CanSee(a.MapPosition()) && !a.IsVendor()
					}); a != nil {
						return a.MapPosition()
					}
					return attacker.MapPosition()
				},
				aoe: aoe,
				infoForTarget: func(p geometry.Point) string {
					a, ok := m.TryGetActorAt(p)
					if !ok || a.d1Monster == nil || attacker.hero == nil {
						return ""
					}
					return fmt.Sprintf("%d%%", attacker.hero.SpellHitChance(attacker.HeroSheet(), a.d1Monster.Level, geometry.DistanceChebyshev(attacker.MapPosition(), p)))
				},
			}
		},
		Description: []util.LabelText{util.LabelTextFromString(f.d1.D.Spells[spell].Name)},
	}
}

// CastD1FromMemory casts a memorized spell: D1 checks the spell level and mana, and takes the mana only if it worked.
func (f *Factory) CastD1FromMemory(caster *Actor, spell int, target geometry.Point) {
	h := caster.hero
	s := caster.HeroSheet()
	if h.SpellLevel(s, spell) <= 0 {
		return
	}
	cost := f.d1.D.ManaCost(h, s, spell)
	mana := caster.GetStats().GetResource(stats.Mana)
	if mana.Value() < cost>>6 || s.Flags&d1.ISplNoMana != 0 && cost > 0 {
		f.logError("Not enough mana.")
		return
	}
	if f.castD1(caster, spell, h.SpellLevel(s, spell), target) {
		mana.Decrease(cost >> 6)
		h.ManaBase -= cost
	}
}

// castD1 casts and ends the player's turn; false if the spell did nothing.
func (f *Factory) castD1(caster *Actor, spell, slvl int, target geometry.Point) bool {
	action := NewSpellAction(caster, spell, slvl, target)
	f.actionBroker.Execute(action)
	if action.Succeeded() && caster == f.state.GetPlayer() {
		f.actionBroker.Execute(NewEndPlayerTurnAction(action.Cause(), action.GetDelayInAut()))
	}
	return action.Succeeded()
}

// d1SpellItemAction makes scrolls and staves castable: they cast at the hero's spell level, without mana (UseMana).
func (f *Factory) d1SpellItemAction(item *Item) {
	it := item.d1
	sd := f.d1.D.Spells[it.Spell]
	use := func(caster *Actor, target geometry.Point) {
		h := caster.hero
		if h == nil {
			return
		}
		s := caster.HeroSheet()
		refund := func() { // the charge was taken (and a used-up scroll removed) before the spell could fail
			item.SetCharges(item.GetCharges() + 1)
			if inv := caster.GetInventory(); !inv.Has(item) && !caster.GetEquipment().IsEquipped(item) {
				inv.Add(item)
			}
		}
		if s.Mag < it.MinMag {
			f.logError(fmt.Sprintf("You need %d magic to use that.", it.MinMag))
			refund()
			return
		}
		if !f.castD1(caster, it.Spell, h.SpellLevel(s, it.Spell), target) {
			refund()
			return
		}
		if it.MaxCharges > 0 {
			it.Charges = max(it.Charges-1, 0)
			item.RefreshD1()
		}
	}
	if d1.SpellTargeted[it.Spell] {
		action := NewTargetedAction(sd.Name, OneTargetTile(f.state.GetMap, util.Arguments{}),
			&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { use(caster, target) },
				Description: []util.LabelText{util.LabelTextFromString(sd.Name)}},
			d1SpellTargeting(f, it.Spell))
		item.SetTargetedAction(action)
	} else {
		action := NewUntargetedAction(sd.Name, item.icon)
		action.SetEffect(&UntargetedEffect{Effect: func(caster *Actor, _ EffectSource) { use(caster, caster.MapPosition()) },
			Description: []util.LabelText{util.LabelTextFromString(sd.Name)}})
		item.SetUntargetedAction(action)
	}
}

// ReadD1Book ports UseItem's IMISC_BOOK: the spell level rises and some mana comes back.
func (f *Factory) readD1Book(user *Actor, it *d1.Item) {
	h := user.hero
	if h == nil {
		return
	}
	if user.HeroSheet().Mag < it.MinMag {
		f.logError(fmt.Sprintf("You need %d magic to read that.", it.MinMag))
		return
	}
	h.SpellLevels[it.Spell] = min(h.SpellLevels[it.Spell]+1, 15)
	sd := f.d1.D.Spells[it.Spell]
	user.GetStats().GetResource(stats.Mana).Increase(float64(sd.Mana))
	f.RefreshD1Spells(user)
	f.logError(fmt.Sprintf("You learn %s (level %d).", sd.Name, h.SpellLevels[it.Spell]))
}

// d1ManaShield ports MI_Manashield for a hit of dam whole points: mana pays first (a third less from spell level 1).
func d1ManaShield(def *Actor, dam int) int {
	if !def.HasStatusEffect(StatusManaShield) || dam <= 0 {
		return dam
	}
	h := def.hero
	if h.SpellLevel(def.HeroSheet(), d1.SplManaShield) > 0 {
		dam -= dam / 3
	}
	mana := def.GetStats().GetResource(stats.Mana)
	if mana.Value() >= dam {
		mana.Decrease(dam)
		return 0
	}
	rest := dam - mana.Value()
	mana.SetToMin()
	def.GetStatusEffects().SetFalse(string(StatusManaShield))
	return rest
}
