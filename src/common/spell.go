package common

import (
	"fmt"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
	"slices"
	"strconv"
)

// D1 spell status effects (decision 6: Mana Shield, Infravision and Stone Curse are status effects).
const (
	StatusStone       StatusEffect = "stone"
	StatusManaShield  StatusEffect = "mana_shield"
	StatusInfravision StatusEffect = "infravision"
)

func spellIcons() map[string]int32 {
	return map[string]int32{
		"STYPE_FIRE": defs.IconEffectFireSmall, "STYPE_LIGHTNING": defs.IconEffectLightning, "STYPE_MAGIC": defs.IconEffectHit,
	}
}

// SpellAction is one cast of a D1 spell, resolved instantly (decision 6).
type SpellAction struct {
	BaseAction
	caster        *Actor
	spell, slvl   int
	target        geometry.Point
	castSucceeded bool
}

func NewSpellAction(caster *Actor, spell, slvl int, target geometry.Point) *SpellAction {
	return &SpellAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
		caster: caster, spell: spell, slvl: slvl, target: target}
}

func (a *SpellAction) GetDelayInAut() int { return CastAut(a.caster) }

// Succeeded: false when the spell found nothing to act on (D1 then doesn't take mana).
func (a *SpellAction) Succeeded() bool { return a.castSucceeded }

func (a *SpellAction) Execute(ctx ActionContext) []Action {
	f := ctx.GameState.factory
	c := &spellCtx{f: f, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), spell: a.spell, slvl: a.slvl,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt64: map[*Actor]int{}, ctx: ctx}
	c.setSpell(a.spell)
	from := c.origin() // Teleport moves the caster
	if ok, handled := c.castFromData(a.target); handled {
		a.castSucceeded = ok
	} else {
		a.castSucceeded = c.cast(a.target)
	}
	if a.castSucceeded {
		sfx := f.Generator.D.Spells[a.spell].Sfx
		playSfxAt(ctx, sfx[0], from)
		playSfxAt(ctx, sfx[1], from)
		ctx.Msg(fmt.Sprintf("You cast %s.", f.Generator.D.Spells[a.spell].Name))
	}
	c.emit(ctx.ActionBroker)
	return c.finish()
}

// CastAut: casting takes the class's spell animation (decision 13).
func CastAut(caster *Actor) int {
	if caster.hero == nil { // only the hero casts D1 spells (NewSpellAction)
		return 0
	}
	return ticksToAUT(caster.hero.Rules().Class(caster.hero.Class).SpellFrames)
}

type spellCtx struct {
	f           *Factory
	ctx         ActionContext
	caster      *Actor
	h           *rules.Hero
	s           rules.Sheet
	spell, slvl int
	m           *gridmap.GridMap[*Actor, *Item, Object]
	cause       Cause
	dealt64     map[*Actor]int // 1/64 points, D1's own unit, converted once per victim in finish
	order       []*Actor
	events      []Event         // what the spell did, emitted when its action finishes
	from        *geometry.Point // where the spell comes from when not the caster: a rune
	heroDealt   int             // whole points the hero's own TARGET_BOTH missiles dealt to him
	lastBolt    int             // 1 + the index of the last bolt in events, 0 for none
	// how a hit resolves, set by whatever is dealing the damage rather than read back off a spell id
	resist         int  // rules.ResistKind*
	undeadOnly     bool // only the undead are harmed (Holy Bolt)
	drainThird     bool // the hit takes a third of what the target has left (Bone Spirit)
	struckAnything bool // a victim was reached at all, hit or missed: tells a whiff from a bolt into thin air
}

// setSpell binds the hit rules of the spell being cast: its resistance kind and the two spells whose hits are
// special. Nothing below strike reads the spell id.
func (c *spellCtx) setSpell(spell int) {
	c.spell = spell
	c.resist = c.f.Generator.D.Spells[spell].Resist
	c.undeadOnly = spell == rules.SplHolyBolt
	c.drainThird = spell == rules.SplBoneSpirit
}

func (c *spellCtx) origin() geometry.Point {
	if c.from != nil {
		return *c.from
	}
	return c.caster.MapPosition()
}

func (c *spellCtx) monsterAt(p geometry.Point) *Actor {
	if a, ok := c.m.TryGetActorAt(p); ok && a.monster != nil && !a.monster.Golem && rules.Fixed(a.GetStats().GetResource(stats.Health).Value()) > c.dealt64[a] {
		return a
	}
	return nil
}

// hitCell is monsterAt for a cell a spell hits: a barrel there breaks first.
func (c *spellCtx) hitCell(p geometry.Point) *Actor {
	c.f.hitObject(c.m, p, c.caster)
	return c.monsterAt(p)
}

func (c *spellCtx) open(p geometry.Point) bool { return c.m.Contains(p) && c.m.IsTransparent(p) }

// chargedBoltCells lists the cells a charged bolt from o wanders through (rules.ChargedBoltFlight) up to the first wall.
func (c *spellCtx) chargedBoltCells(o, target geometry.Point, rnd int) []geometry.Point {
	var cells []geometry.Point
	p := o
	for _, t := range rules.ChargedBoltFlight(DirTo(o, target), rnd) {
		next := o.Add(geometry.Point{X: t[0], Y: t[1]})
		if next == p {
			continue
		}
		if !c.open(next) {
			break
		}
		p = next
		cells = append(cells, p)
	}
	return cells
}

// ray lists the cells from `from` (excluded) through `to` and on, up to maxLen cells or the first wall.
func (c *spellCtx) ray(from, to geometry.Point, maxLen int) []geometry.Point {
	cells, _, _ := c.rayToWall(from, to, maxLen)
	return cells
}

// rayToWall is ray, plus the wall cell that stopped it, if one did.
func (c *spellCtx) rayToWall(from, to geometry.Point, maxLen int) (cells []geometry.Point, wall geometry.Point, hitWall bool) {
	d := to.Sub(from)
	n := max(geometry.Abs(d.X), geometry.Abs(d.Y))
	if n == 0 {
		return nil, wall, false
	}
	k := (maxLen + n - 1) / n
	far := from.Add(geometry.Point{X: d.X * k, Y: d.Y * k})
	for _, p := range geometry.LineOfSight(from, far, func(p geometry.Point) bool { return p == from || c.open(p) })[1:] {
		if len(cells) == maxLen {
			break
		}
		if !c.open(p) {
			return cells, p, c.m.Contains(p)
		}
		cells = append(cells, p)
	}
	return cells, wall, false
}

func isSolidObjectAt(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) bool {
	o := objectAt(m, p)
	return o != nil && !o.IsWalkable(nil)
}

// lineClear ports LineClear/CheckBlock: no wall between the two cells.
func (c *spellCtx) lineClear(from, to geometry.Point) bool {
	los := geometry.LineOfSight(from, to, func(p geometry.Point) bool { return p == from || p == to || c.open(p) })
	return los[len(los)-1] == to
}

// freeNear ports the CrawlTable searches: the first cell of rings 0-5, in 1.09's order, that is ok.
func (c *spellCtx) freeNear(target geometry.Point, ok func(geometry.Point) bool) (geometry.Point, bool) {
	for _, ring := range rules.CrawlRings[:6] {
		for _, o := range ring {
			if p := target.Add(geometry.Point{X: o[0], Y: o[1]}); c.m.Contains(p) && ok(p) {
				return p, true
			}
		}
	}
	return geometry.Point{}, false
}

func (c *spellCtx) emptyFloor(p geometry.Point) bool {
	return c.m.IsCurrentlyPassable(p) && !c.m.IsObjectAt(p) && c.lineClear(c.origin(), p)
}

// HasInfravision: the spell, the Murky Pool, or an item with the infravision affix (_pInfraFlag).
func (a *Actor) HasInfravision() bool {
	return a.HasStatusEffect(StatusInfravision) || a.hero != nil && a.HeroSheet().Flags&rules.ISplInfravision != 0
}

func resistBits(kind int) (resist, immune int) {
	switch kind {
	case rules.ResistKindFire:
		return rules.ResistFire, rules.ImmuneFire
	case rules.ResistKindLightning:
		return rules.ResistLightning, rules.ImmuneLightning
	case rules.ResistKindMagic:
		return rules.ResistMagic, rules.ImmuneMagic
	}
	return 0, 0
}

// strike ports MonsterMHit for a player's spell: `ticks` hit rolls of dam64 (1/64) each.
func (c *spellCtx) strike(a *Actor, dam64, ticks int) bool {
	mon := a.monster
	resist, immune := resistBits(c.resist)
	d := c.f.Generator.D
	if untouchable(d, mon, false) || mon.MagicRes&immune != 0 || c.undeadOnly && mon.Data.Class != "MC_UNDEAD" && !mon.Brain(d).ArchBoss {
		return false
	}
	if c.drainThird {
		dam64 = (rules.Fixed(a.GetStats().GetResource(stats.Health).Value()) - c.dealt64[a]) / 3
	}
	chance := c.h.SpellHitChance(c.s, mon.Level)
	total := 0
	for i := 0; i < ticks; i++ {
		if a.HasStatusEffect(StatusStone) || rng.Intn(100) < chance {
			total += dam64
		}
	}
	c.struckAnything = true
	if total == 0 { // a spell can miss (MonsterMHit's to-hit roll); say so, or the bolt just fizzles silently
		c.say(fmt.Sprintf("Your spell misses %s.", a.Name()))
		return false
	}
	if mon.MagicRes&resist != 0 {
		total >>= 2
	}
	alert(a, c.caster)
	if _, seen := c.dealt64[a]; !seen {
		c.order = append(c.order, a)
	}
	c.dealt64[a] += total
	c.flash(a.MapPosition(), 0.3, defs.IconEffectHit)
	return true
}

// lastMissile is the spell's last bolt so far.
func (c *spellCtx) lastMissile() (Missile, bool) {
	if c.lastBolt == 0 {
		return Missile{}, false
	}
	return c.events[c.lastBolt-1].(Missile), true
}

func (c *spellCtx) flash(p geometry.Point, seconds float64, icon int32) {
	c.events = append(c.events, Flash{At: p, Seconds: seconds, Icon: icon})
}

// missile is a straight flight (or one through several cells) of the spell's.
func (c *spellCtx) missile(by *Actor, path []geometry.Point, secPerTile float64, icon int32) {
	c.events = append(c.events, Missile{By: by, Path: path, SecPerTile: secPerTile, Icon: icon})
}

// emit hands the spell's events to the broker.
func (c *spellCtx) emit(b *Broker) {
	for _, ev := range c.events {
		b.Emit(ev)
	}
}

func (c *spellCtx) finish() []Action {
	var actions []Action
	if player := c.f.State.GetPlayer(); c.heroDealt > 0 && player.IsAlive() {
		actions = append(actions, NewChangeResourceAction(c.cause, player, stats.Health, -float64(c.heroDealt)))
	}
	for _, a := range c.order {
		// one conversion, at the one place D1's 1/64 meets the engine's whole hit points. A blow that landed
		// is never nothing: a resisted firebolt is a quarter of a few points, which used to floor to zero
		// damage while still costing the mana, and read as the monster being immune.
		if c.dealt64[a] <= 0 {
			continue
		}
		dam := max(rules.Whole(c.dealt64[a]), 1)
		c.say(fmt.Sprintf("Your spell hits %s for %d damage.", a.Name(), dam))
		actions = append(actions, NewChangeResourceAction(c.cause, a, stats.Health, -float64(dam)))
	}
	return actions
}

// say logs what a cast did, but only for the hero's own spells: a trap's or a rune's missile is nobody's spell.
func (c *spellCtx) say(msg string) {
	if c.f == nil || c.f.Msg == nil || c.f.State == nil || c.caster == nil || c.caster != c.f.State.GetPlayer() {
		return
	}
	c.f.Msg(msg)
}

// hurtHero ports CheckMissileCol's player half for the hero's own TARGET_BOTH missiles (Hellfire's walls and rune
// blasts): the hero standing at p takes rolls like a trap's, as PlayerMHit has no monster to take a level from.
func (c *spellCtx) hurtHero(p geometry.Point, mind, maxd int, shift bool, ticks int) {
	player := c.f.State.GetPlayer()
	if player.hero == nil || player.MapPosition() != p {
		return
	}
	mis := MisFlashTrap // a magic missile
	switch c.resist {
	case rules.ResistKindFire:
		mis = MisFireWall
	case rules.ResistKindLightning:
		mis = MisLightCtrl
	}
	mc := &missileCtx{ctx: c.ctx, from: c.origin(), player: player, c: c}
	mc.hit(mis, p, c.origin(), mind, maxd, shift, ticks)
	c.heroDealt += mc.dealt
}

func (c *spellCtx) secPerTile() float64 {
	return rules.MissileSecondsPerTile(rules.SpellMissileVel(c.spell, c.slvl))
}

func (c *spellCtx) icon() int32 { return spellIcons()[c.f.Generator.D.Spells[c.spell].Type] }

// bolt flies from `from` towards `to` until it hits a monster or a wall; a miss flies on (CheckMissileCol).
// It returns where it stopped: the monster's, solid object's or wall's cell (a wall stops a missile on it), else
// the last cell it reached, and whether something stopped it. Barrels in its way break.
func (c *spellCtx) bolt(from, to geometry.Point, hit func(a *Actor) bool) (impact geometry.Point, stopped bool) {
	last := from
	cells, wall, hitWall := c.rayToWall(from, to, 20)
	impact, stopped = wall, hitWall
	for _, p := range cells {
		last = p
		if o := objectAt(c.m, p); o != nil && !o.IsWalkable(nil) {
			c.f.hitObject(c.m, p, c.caster)
			impact, stopped = p, true
			break
		}
		if a := c.monsterAt(p); a != nil && hit(a) {
			impact, stopped = p, true
			break
		}
	}
	if !stopped {
		impact = last
	}
	if b, ok := c.lastMissile(); ok && b.Path[len(b.Path)-1] == from { // Elemental's homing leg flies on from where the first one stopped
		b.Path = append(b.Path, last)
		c.events[c.lastBolt-1] = b
	} else if sfx := c.f.Generator.D.Spells[c.spell].Sfx[2]; last != from {
		c.events = append(c.events, Missile{By: c.caster, Path: []geometry.Point{from, last}, SecPerTile: c.secPerTile(), Icon: c.icon(), ArriveSfx: sfx})
		c.lastBolt = len(c.events)
	} else {
		playSfxAt(c.ctx, sfx, last)
	}
	return impact, stopped
}

func (c *spellCtx) payHP(hp int) {
	c.caster.GetStats().GetResource(stats.Health).Decrease(hp)
	if !c.caster.IsAlive() {
		c.f.ActionBroker.QueueForExecution(NewDieAction(c.cause, c.caster))
	}
}

// castFromData resolves the spell through the one effect model: the hit zone, on-hit effect and targeting rules
// its content declares. handled is false for a spell whose content carries no mapping yet, which then falls back
// to the switch below. Nothing here knows which spell it is casting.
func (c *spellCtx) castFromData(target geometry.Point) (ok, handled bool) {
	declared := c.f.Generator.D.Spells[c.spell].Effect
	if len(declared) == 0 {
		return false, false
	}
	if !c.castAllowed(target) {
		return false, true
	}
	var effects []*HitEffect
	for _, effectName := range declared {
		name, args := effectName, util.Arguments{}
		if util.LooksLikeAFunction(effectName) {
			name, args = util.GetNameAndArgs(effectName)
		}
		// a roll grows with the spell's level, which only the cast knows
		effect := c.f.NewOnHitEffectFromNameAndArgs(name, append(args, strconv.Itoa(c.slvl)))
		if effect == nil {
			return false, true
		}
		effects = append(effects, effect)
	}
	if hp := c.f.Generator.D.Spells[c.spell].HealthCost; hp > 0 {
		c.payHP(hp)
	}
	zone := HitZoneFromString(c.f.State.GetMap, c.f.Generator.D.Spells[c.spell].HitZone)
	positions := zone.GetHitPositions(c.origin(), target)
	for _, effect := range effects {
		effect.Activate(c.caster, target, positions)
	}
	return true, true
}

// castAllowed holds the two checks every cast makes: the spell has to work here, and a spell that needs a victim
// needs one other than the caster.
func (c *spellCtx) castAllowed(target geometry.Point) bool {
	if c.f.State.Level(c.m.GetName()).Town && !c.f.Generator.D.Spells[c.spell].Town {
		c.ctx.Msg("That spell doesn't work in town.")
		c.f.PlayHeroSound(27)
		return false
	}
	if sd := c.f.Generator.D.Spells[c.spell]; sd.Targeted && !sd.SelfTarget && target == c.origin() {
		c.ctx.Msg("You need a target other than yourself.")
		return false
	}
	return true
}

func (c *spellCtx) cast(target geometry.Point) bool {
	f, h, slvl := c.f, c.h, c.slvl
	clvl := h.Level
	if !c.castAllowed(target) {
		return false
	}
	switch c.spell {
	case rules.SplIdentify:
		item := f.identifyTarget
		if item == nil {
			return false
		}
		item.roll.Identified = true
		f.ActionBroker.Emit(ItemIdentified{Item: item.roll})
		item.Refresh()
		SyncHero(c.caster)
		f.Msg(fmt.Sprintf("It is %s.", item.Name()))
		if f.Show != nil {
			f.Show(ShowItem{item})
		}
	case rules.SplJester: // missiles_jester: one of nine spells at the Jester's spell level
		c.spell = [10]int{rules.SplFirebolt, rules.SplFirebolt, rules.SplFireball, rules.SplFireWall, rules.SplGuardian, rules.SplChain,
			rules.SplTown, rules.SplTeleport, rules.SplApocalypse, rules.SplStone}[rng.Intn(10)]
		c.setSpell(c.spell)
		c.ctx.Msg(fmt.Sprintf("The Jester casts %s.", f.Generator.D.Spells[c.spell].Name))
		return c.cast(target)
	case rules.SplWarp: // missiles_warp: to the nearest stairs, beside them
		return f.warp(c)
	case rules.SplTown:
		return f.openPortal(c, target)
	case rules.SplGuardian:
		p, ok := c.freeNear(target, c.emptyFloor)
		if !ok {
			return false
		}
		f.spawnGuardian(c.caster, p, rules.GuardianTurns(clvl, slvl))
	case rules.SplGolem:
		return f.summonGolem(c, target)
	case rules.SplChain: // MI_Chain: a Lightning bolt at the target and one at every monster within slvl+2 CrawlTable rings
		ray := RayAoE(func() *gridmap.GridMap[*Actor, *Item, Object] { return c.m }, util.Arguments{"112"})
		o := c.origin()
		cells := ray.GetHitPositions(o, target)
		for _, ring := range rules.CrawlRings[1:min(slvl+3, 19)] {
			for _, off := range ring {
				if t := o.Add(geometry.Point{X: off[0], Y: off[1]}); c.m.Contains(t) && c.monsterAt(t) != nil {
					cells = append(cells, ray.GetHitPositions(o, t)...)
				}
			}
		}
		f.ActionBroker.Execute(NewSpellDamageAction(c.caster, "lightning", rules.ResistKindLightning, "per_cell", slvl, target, cells))
	case rules.SplWave: // MI_Wave: a wall of flames across the aim, each rolling on until a wall or the first monster
		sd := DirTo(c.origin(), target)
		fwd := geometry.Dirs[sd]
		blocked := func(p geometry.Point) bool { return !c.m.Contains(p) || !c.m.IsTransparent(p) }
		var cells []geometry.Point
		roll := func(p geometry.Point) {
			for n := 0; n < 112 && !blocked(p); n++ {
				cells = append(cells, p)
				if c.monsterAt(p) != nil {
					return
				}
				p = p.Add(fwd)
			}
		}
		center := c.origin().Add(fwd)
		if !blocked(center) {
			roll(center)
			for _, side := range []geometry.Point{geometry.Dirs[(sd+6)%8], geometry.Dirs[(sd+2)%8]} {
				for j, p := 0, center.Add(side); j < slvl/2+2 && !blocked(p); j, p = j+1, p.Add(side) {
					roll(p)
				}
			}
		}
		f.ActionBroker.Execute(NewSpellDamageAction(c.caster, "wave", rules.ResistKindFire, "per_cell", slvl, target, cells))
	case rules.SplManaShield:
		raiseManaShield(c.caster, slvl)
	case rules.SplStone: // AddStone: the first monster of six CrawlTable rings, never Diablo; no mana without one
		p, ok := c.freeNear(target, func(p geometry.Point) bool {
			a := c.monsterAt(p)
			return a != nil && !a.monster.Brain(c.f.Generator.D).ArchBoss
		})
		if !ok {
			return false
		}
		turns := min(slvl+6, 15) << 4 / rules.RefWalkTicks
		f.ActionBroker.Execute(NewApplyStatusAction(c.cause, c.monsterAt(p), StatusStone, turns))
	case rules.SplTeleport: // AddTeleport: the first cell of six CrawlTable rings with no wall, monster, object or player
		p, ok := c.freeNear(target, func(p geometry.Point) bool {
			return c.m.IsWalkable(p) && !c.m.IsActorAt(p) && !c.m.IsObjectAt(p)
		})
		if !ok {
			return false
		}
		move := NewMoveAction(c.cause, c.caster, p, c.caster == f.State.GetPlayer())
		move.DisableAnimation()
		f.ActionBroker.Execute(move)
	case rules.SplTelekinesis:
		return f.telekinesis(c, target)
	default:
		return false
	}
	return true
}

// spawnFireWall places one cell of a fire wall (AddFirewall, MI_Firewall), for Fire Wall or Ring of Fire.
// Hellfire's burns the hero too, dlvl·16 ticks longer, and for a flat 8·clvl/64 a tick (AddFirewall's bug).
func (f *Factory) spawnFireWall(caster *Actor, p geometry.Point, slvl, layers int) {
	clvl := caster.hero.Level
	turns := rules.FireWallTurns(slvl, f.State.CurrentLevel().Depth)
	f.spawnSpellWall(caster, p, rules.ResistKindFire, turns, true, layers, defs.IconEffectFireSmall, func() int { return rules.HellfireFireWallDamage64(clvl) })
}

// spawnLightningWall places one cell of a lightning wall (MIS_LIGHTWALL): 255·(slvl+1) ticks of 16·(2d10 + clvl
// + 2)/64 a tick, rolled once when it is laid, on monsters and the hero alike.
func (f *Factory) spawnLightningWall(caster *Actor, p geometry.Point, slvl, clvl, layers int) {
	dam := 16 * (rng.Intn(10) + rng.Intn(10) + clvl + 2) // one roll a controller tick, shared by what it lays
	f.spawnSpellWall(caster, p, rules.ResistKindLightning, 255*(slvl+1)/rules.RefWalkTicks, true, layers, defs.IconEffectLightning, func() int { return dam })
}

// spawnSpellWall places one wall cell of `layers` walls stacked (a fire wall's start cell holds two), each rolling
// dam64() once as it is laid: each turn every layer hits the monster standing in it, and with both the hero too,
// for its 1/64 a tick over a turn's 8 ticks.
func (f *Factory) spawnSpellWall(caster *Actor, p geometry.Point, resist, turns int, both bool, layers int, icon int32, dam64 func() int) {
	m := f.State.GetMap()
	if m.IsObjectAt(p) || turns <= 0 {
		return
	}
	dams := make([]int, layers)
	for i := range dams {
		dams[i] = dam64()
	}
	wall := NewFire(0)
	wall.icon, wall.turnsUntilRemoval, wall.FireWall = icon, turns, resist == rules.ResistKindFire
	wall.SetDealDamage(func(location geometry.Point) {})
	wall.SetOnTurn(func() {
		if !wall.IsAlive() || f.State.GetMap() != m {
			m.RemoveObject(wall)
			wall.turnsUntilRemoval = 0
			return
		}
		a, ok := m.TryGetActorAt(p)
		if !ok || !caster.IsAlive() || a.monster == nil && !(both && a == f.State.GetPlayer()) {
			return
		}
		f.ActionBroker.Execute(&spellTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			caster: caster, resist: resist, run: func(c *spellCtx) {
				c.from = &p
				for _, d := range dams {
					if v := c.monsterAt(p); v != nil {
						c.strike(v, d, rules.RefWalkTicks)
					} else if both {
						c.hurtHero(p, d, d, true, rules.RefWalkTicks)
					}
				}
			}})
	})
	f.GetTimeKeeper().AddTimed(wall)
	m.AddObject(wall, p)
}

// spawnGuardian places a Guardian: every other turn it spits a firebolt at the first monster its scan meets (MI_Guardian).
func (f *Factory) spawnGuardian(caster *Actor, p geometry.Point, turns int) {
	m := f.State.GetMap()
	g := NewFire(0)
	g.icon, g.frameCount, g.turnsUntilRemoval = defs.IconEffectFireBig, 2, turns
	g.SetDealDamage(func(geometry.Point) {})
	g.SetOnTurn(func() {
		if !g.IsAlive() || f.State.GetMap() != m {
			m.RemoveObject(g)
			g.turnsUntilRemoval = 0
			return
		}
		if g.turnsUntilRemoval%2 != 0 || caster.hero == nil {
			return
		}
		f.ActionBroker.Execute(&spellTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			caster: caster, spell: rules.SplFirebolt, run: func(c *spellCtx) {
				var best *Actor // Sentfire: the first monster in line along the scan
				for _, o := range rules.GuardianScan {
					if t := p.Add(geometry.Point{X: o[0], Y: o[1]}); m.Contains(t) && c.monsterAt(t) != nil && c.lineClear(p, t) {
						best = c.monsterAt(t)
						break
					}
				}
				if best != nil {
					dam := rules.FireboltDamage(rng, c.s.Mag, c.h.SpellLevel(c.s, rules.SplFirebolt))
					playSfxAt(c.ctx, c.f.Generator.D.Spells[rules.SplFirebolt].Sfx[1], p)
					c.bolt(p, best.MapPosition(), func(a *Actor) bool { return c.strike(a, rules.Fixed(dam), 1) })
				}
			}})
	})
	f.GetTimeKeeper().AddTimed(g)
	m.AddObject(g, p)
}

// spellTickAction resolves the damage of a lingering spell (fire wall, guardian) on its turn.
type spellTickAction struct {
	BaseAction
	caster *Actor
	spell  int // only for the runes, which cast their own spell through the content mapping
	resist int // rules.ResistKind* of the damage this tick deals, when it is not a whole spell's
	run    func(c *spellCtx)
}

func (a *spellTickAction) GetDelayInAut() int { return 0 }
func (a *spellTickAction) Execute(ctx ActionContext) []Action {
	c := &spellCtx{f: ctx.GameState.factory, ctx: ctx, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), spell: a.spell,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt64: map[*Actor]int{}}
	c.setSpell(a.spell)
	if a.resist != 0 {
		c.resist = a.resist
	}
	a.run(c)
	c.emit(ctx.ActionBroker)
	return c.finish()
}

// movePlayer puts the player on a cell (Teleport, Phasing, the Holy shrine).
func (f *Factory) movePlayer(player *Actor, p geometry.Point, why ActionName) {
	m := f.State.GetMap()
	m.MoveActor(player, p)
	player.SetDrawPosition(p.ToPointF())
	f.ActionBroker.QueueForExecution(NewUpdateFoVAction(NewActorCause(why, player), player))
}

// randomTeleport ports MIS_RNDTELEPORT: a jump of 4-6 cells along each axis.
func (f *Factory) randomTeleport(player *Actor) {
	m, p := f.State.GetMap(), player.MapPosition()
	for try := 0; try < 500; try++ {
		dx, dy := rng.Intn(3)+4, rng.Intn(3)+4
		if rng.Intn(2) == 1 {
			dx = -dx
		}
		if rng.Intn(2) == 1 {
			dy = -dy
		}
		if q := p.Add(geometry.Point{X: dx, Y: dy}); m.CanPlaceActorHere(q) && !m.IsObjectAt(q) {
			f.movePlayer(player, q, "Phasing")
			return
		}
	}
}

// pickIdentifyTarget lists the unidentified items before Identify costs anything; picking one runs cast with it
// as the target, so cancelling the list keeps the scroll or mana (D1's cursor mode can be cancelled too).
func (f *Factory) pickIdentifyTarget(player *Actor, cast func()) {
	var items []*Item
	for _, it := range heroItems(player) {
		if it.roll != nil && it.roll.Magical != 0 && !it.roll.Identified {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		f.Msg("You have nothing to identify.")
		return
	}
	if f.Show != nil {
		openItemList(f.Show, player, items, func(item *Item) {
			f.identifyTarget = item
			cast()
			f.identifyTarget = nil
		})
	}
}

// telekinesis ports the telekinesis cursor: operate an object, fetch an item, or push a monster away.
func (f *Factory) telekinesis(c *spellCtx, target geometry.Point) bool {
	m, player := c.m, c.caster
	if !player.CanSee(target) {
		return false
	}
	if o := objectAt(m, target); o != nil {
		f.operateObject(o, player)
		return true
	}
	if it, ok := m.TryGetItemAt(target); ok {
		m.RemoveItem(it)
		if it.roll != nil && it.roll.Type == rules.ITypeGold {
			player.GetStats().IncrementBy(stats.Gold, it.roll.Value)
		} else {
			player.GetInventory().Add(it)
		}
		c.ctx.Msg(fmt.Sprintf("You pick up %s.", it.Name()))
		return true
	}
	if a := c.monsterAt(target); a != nil && !a.HasStatusEffect(StatusStone) {
		mon := a.monster
		if mon.IsTalker(c.f.Generator.D) || mon.TalkMsg != "" { // DoTelekinesis leaves M_Talker and mtalkmsg monsters be
			return true
		}
		// M_GetKnockback: a step back against its own facing, into hit recovery
		if p := target.Add(geometry.Dirs[(mon.Dir+4)%8]); m.CanPlaceActorHere(p) {
			m.MoveActor(a, p)
			a.SetDrawPosition(p.ToPointF())
			if lost := MonsterRecoveryAUT(mon); lost > 0 && a.GetEnergy() > -lost {
				a.RemoveEnergy(a.GetEnergy() + lost)
			}
			mon.LastMode, mon.StandTicks = ModeGotHit, 0
		}
		return true
	}
	return false
}

// Brain is the monsters.rec record of the monster's AI.
func (m *Monster) Brain(d *rules.Data) rules.MonsterAI { return d.MonsterAIs[m.AI] }

// IsTalker is M_Talker.
func (m *Monster) IsTalker(d *rules.Data) bool { return m.Brain(d).Talker }

// summonGolem ports AddGolem/SpawnGolum: the old golem dies (M_StartKill, corpse and all), a new one appears
// near the target. The mana is spent even when MI_Golem then finds no free cell.
func (f *Factory) summonGolem(c *spellCtx, target geometry.Point) bool {
	for _, a := range c.m.Actors() {
		if a.monster != nil && a.monster.Golem && a.IsAlive() {
			c.order, c.dealt64[a] = append(c.order, a), rules.Fixed(a.GetStats().GetResource(stats.Health).Value())
		}
	}
	p, ok := c.freeNear(target, c.emptyFloor)
	if !ok {
		return true
	}
	gs := rules.GolemStats(c.slvl, c.h.Level, c.s.MaxMana)
	f.placeGolem(c.m, p, SavedGolem{AC: gs.AC, Hit: gs.Hit, MinDam: gs.MinDam, MaxDam: gs.MaxDam, MaxHP: gs.HP, HP: gs.HP})
	return true
}

// SavedGolem is the player's golem, which follows them from level to level and into the save.
type SavedGolem struct {
	AC, Hit, MinDam, MaxDam, MaxHP, HP int
	ID                                 uint64 `json:",omitempty"` // Actor.ID
}

func (f *Factory) placeGolem(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point, g SavedGolem) *Actor {
	a := f.newMonster(f.Generator.D.MonsterByTag("golem"), nil) // the player's own: no Nightmare scaling
	a.isHostile = false
	mon := a.monster
	mon.Golem, mon.SpawnIndex, mon.AC, mon.Hit, mon.MinDam, mon.MaxDam = true, -1, g.AC, g.Hit, g.MinDam, g.MaxDam
	a.SetColor(defs.White)
	a.GetStats().SetAttribute(stats.MaxHealth, g.MaxHP)
	SetResource(a.GetStats().GetResource(stats.Health), max(g.HP, 1))
	a.SetDrawPosition(p.ToPointF())
	a.SetID(g.ID)
	m.AddActor(a, p)
	f.State.OnActorSpawned(a)
	return a
}

// LivingGolem is the player's living golem on m, if any.
func LivingGolem(m *gridmap.GridMap[*Actor, *Item, Object]) *Actor {
	for _, a := range m.Actors() {
		if a.monster != nil && a.monster.Golem && a.IsAlive() {
			return a
		}
	}
	return nil
}

// SaveGolem describes a golem for moving or saving it.
func SaveGolem(a *Actor) SavedGolem {
	mon, st := a.monster, a.GetStats()
	return SavedGolem{AC: mon.AC, Hit: mon.Hit, MinDam: mon.MinDam, MaxDam: mon.MaxDam, MaxHP: st.GetAttribute(stats.MaxHealth), HP: st.GetResource(stats.Health).Value(), ID: a.id}
}

// PlaceGolemNear puts the player's golem back next to the player (after a level change or loading a save).
func (f *Factory) PlaceGolemNear(player *Actor, g SavedGolem) {
	m := f.State.GetMap()
	if cells := freeCellsNear(m, player.MapPosition(), 1, m.CanPlaceActorHere); len(cells) > 0 {
		f.placeGolem(m, cells[0], g)
	}
}

// Portal is the open town portal: where it stands in the dungeon.
type Portal struct {
	Map  string // the level it was opened on
	Town string // the town it leads to: the deepest visited when it opened
	X, Y int
}

const portalLocation = "portal"

type portalObject struct {
	pos    geometry.Point
	target gridmap.Transition
}

func (o *portalObject) Name() string                    { return "Town Portal" }
func (o *portalObject) ID() string                      { return "town_portal" }
func (o *portalObject) MapPosition() geometry.Point     { return o.pos }
func (o *portalObject) SetMapPosition(p geometry.Point) { o.pos = p }
func (o *portalObject) Icon(tick uint64) int32 {
	return []int32{defs.Icon("prop.portal"), defs.Icon("prop.portal.2")}[util.GetLoopingFrameFromTick(tick, 0.3, 2)]
}
func (o *portalObject) IsWalkable(*Actor) bool        { return true }
func (o *portalObject) IsTransparent() bool           { return true }
func (o *portalObject) IsPassableForProjectile() bool { return true }
func (o *portalObject) Turn()                         {}
func (o *portalObject) ActorWalkedOn(*Actor)          {}
func (o *portalObject) ActorBumpedInto(*Actor)        {}

func (s *State) Portal() *Portal     { return s.portal }
func (s *State) SetPortal(p *Portal) { s.portal = p }

func (f *Factory) openPortal(c *spellCtx, target geometry.Point) bool {
	m := f.State.GetMap()
	if old := f.State.portal; old != nil {
		f.State.ClosePortal()
	}
	// AddTown: the first cell of six CrawlTable rings around the target with no wall, object (missiles are ours),
	// player, or stairs within a cell (CheckIfTrig); monsters don't matter
	p, ok := c.freeNear(target, func(p geometry.Point) bool {
		if !m.IsWalkable(p) || m.IsObjectAt(p) || p == f.State.GetPlayer().MapPosition() {
			return false
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if _, trig := m.GetTransitionAt(p.Add(geometry.Point{X: dx, Y: dy})); trig {
					return false
				}
			}
		}
		return true
	})
	if !ok {
		return false
	}
	f.State.portal = &Portal{Map: m.GetName(), Town: f.State.DeepestTown(), X: p.X, Y: p.Y}
	PlacePortal(f.State, m)
	return true
}

// PlacePortal puts the town portal's two ends on a map when it has one: at the dungeon spot, or at the town's
// "portal" Location the town map draws.
func PlacePortal(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	p := s.portal
	if p == nil {
		return
	}
	var pos geometry.Point
	var target gridmap.Transition
	switch {
	case m.GetName() == p.Map:
		pos, target = geometry.Point{X: p.X, Y: p.Y}, gridmap.Transition{TargetMap: p.Town, TargetLocation: portalLocation}
	case m.GetName() == p.Town:
		pos, target = m.GetNamedLocation(portalLocation), gridmap.Transition{TargetMap: p.Map, TargetLocation: portalLocation}
	default:
		return
	}
	if m.IsObjectAt(pos) {
		return
	}
	m.AddNamedLocation(portalLocation, pos)
	m.AddObject(&portalObject{pos: pos, target: target}, pos)
	m.AddTransitionAt(pos, target)
	delete(s.glowCache, m) // the portal is a light source
}

// ClosePortal removes both ends of the town portal from the maps that are loaded.
func (s *State) ClosePortal() {
	if s.portal == nil {
		return
	}
	maps := []*gridmap.GridMap[*Actor, *Item, Object]{s.Map(s.portal.Map), s.Map(s.portal.Town)}
	for _, m := range maps {
		if m == nil {
			continue
		}
		for _, obj := range append([]Object(nil), m.Objects()...) {
			if po, ok := obj.(*portalObject); ok {
				m.RemoveObject(po)
				m.RemoveTransitionAt(po.pos)
				delete(s.glowCache, m)
			}
		}
	}
	s.portal = nil
}

// SpellSkills lists the player's memorized spells as skills (the skill bar and the spell menu use them).
func (f *Factory) SpellSkills(player *Actor) ([]*TargetedAction, []*UntargetedAction) {
	h := player.hero
	if h == nil {
		return nil, nil
	}
	s := player.HeroSheet()
	var targeted []*TargetedAction
	var untargeted []*UntargetedAction
	if t, u := f.classSkill(player); t != nil {
		t.SetInternalName(classSkillID)
		targeted = append(targeted, t)
	} else if u != nil {
		u.SetInternalName(classSkillID)
		untargeted = append(untargeted, u)
	}
	for spell := range h.SpellLevels {
		if h.SpellLevel(s, spell) <= 0 {
			continue
		}
		sd := f.Generator.D.Spells[spell]
		sp := spell
		// only the name on the skill bar - the sidebar is too narrow for more; level and mana go in the tooltip
		slvl := h.SpellLevel(s, spell)
		desc := spellTooltip(f.Generator.D.Spells[sp].Description, f.Generator.D.SpellDamageText(sp, s.Mag, h.Level, slvl), slvl, rules.Whole(f.Generator.D.ManaCost(h, s, spell)))
		if f.Generator.D.Spells[spell].Targeted {
			action := NewTargetedAction(sd.Name, OneTargetTile(f.State.GetMap, util.Arguments{}),
				&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { f.CastFromMemory(caster, sp, target) },
					Description: desc},
				f.skillTargeting(sp, slvl))
			action.SetIcon(f.spellIcon(sp))
			action.SetInternalName(sd.ID)
			targeted = append(targeted, action)
		} else {
			action := NewUntargetedAction(sd.Name, f.spellIcon(sp))
			action.SetEffect(&UntargetedEffect{Effect: func(caster *Actor, _ EffectSource, _ bool) { f.CastFromMemory(caster, sp, caster.MapPosition()) },
				Description: desc})
			action.SetInternalName(sd.ID)
			untargeted = append(untargeted, action)
		}
	}
	return targeted, untargeted
}

// spellIcon is the spell's icon: the mod's spell.<id>, or for a named skill without one the icon its record names.
func (f *Factory) spellIcon(spell int) int32 {
	sd := f.Generator.D.Spells[spell]
	if icon, ok := defs.IconOK(defs.IconID("spell", sd.ID)); ok || spell >= rules.NumSpells {
		if !ok {
			return sd.Icon
		}
		return icon
	}
	return defs.Icon(defs.IconID("spell", sd.ID)) // a D1 spell the mod has no icon for stops the game here
}

// skillTargeting aims a spell: like a bolt, unless its record names targeting rules, which get its level appended.
func (f *Factory) skillTargeting(spell, slvl int) *TargetRulesProvider {
	rule := f.Generator.D.Spells[spell].Targeting
	if rule == "" {
		return spellTargeting(f, spell)
	}
	name, args := rule, util.Arguments{}
	if util.LooksLikeAFunction(rule) {
		name, args = util.GetNameAndArgs(rule)
	}
	if resolver, ok := GetAllTargetingRules()[name]; ok {
		return resolver(append(args, strconv.Itoa(slvl)))
	}
	return spellTargeting(f, spell)
}

// spellTooltip is a memorized spell's popup: what it does, its damage, then its spell level and what a cast costs.
func spellTooltip(text, damage string, slvl, mana int) []util.LabelText {
	lines := []util.LabelText{util.LabelTextFromStringWithColor(text, defs.OffWhite), util.LabelTextFromString("")}
	if damage != "" {
		lines = append(lines, util.LabelTextFromStringWithColor(damage, defs.LightGray))
	}
	lines = append(lines, util.LabelTextFromStringWithColor(fmt.Sprintf("Spell level %d", slvl), defs.LightGray))
	cost := fmt.Sprintf("Mana cost: %d", mana)
	if mana == 0 {
		cost = "Costs no mana"
	}
	return append(lines, util.LabelTextFromStringWithColor(cost, defs.LightGray))
}

// classSkillID is the class skill's internal name: a hero has exactly one.
const classSkillID = "CLASS_SKILL"

// RefreshSpells updates the player's spell skills after learning or changing gear. A skill the player already
// had keeps its pointer (updated in place), so the belt and anything else holding it stays current.
func (f *Factory) RefreshSpells(player *Actor) {
	t, u := f.SpellSkills(player)
	player.SetTargetedSkills(keepIdentity(player.GetTargetedSkills(), t, (*TargetedAction).GetInternalName))
	player.SetUntargetedSkills(keepIdentity(player.GetUntargetedSkills(), u, (*UntargetedAction).GetInternalName))
}

// keepIdentity swaps each fresh entry for the old one with the same name, overwritten with the fresh contents.
func keepIdentity[T any](old, fresh []*T, name func(*T) string) []*T {
	for i, n := range fresh {
		if at := slices.IndexFunc(old, func(o *T) bool { return name(o) == name(n) }); at >= 0 {
			*old[at] = *n
			fresh[i] = old[at]
		}
	}
	return fresh
}

func spellTargeting(f *Factory, spell int) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, aoe *AoEResolver, m *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &RangedTargetingRules{
				attacker: attacker,
				isOnMap:  m.Contains,
				targets: func() []geometry.Point {
					return targetsByDistance(m, attacker.MapPosition(), func(a *Actor) bool {
						return a.IsVisible() && a.IsHostileTo(attacker) && attacker.CanSee(a.MapPosition()) && !a.IsVendor() &&
							boltReaches(m, attacker.MapPosition(), a.MapPosition())
					})
				},
				aoe: aoe,
				infoForTarget: func(p geometry.Point) string {
					a, ok := m.TryGetActorAt(p)
					if !ok || a.monster == nil || attacker.hero == nil {
						return ""
					}
					return fmt.Sprintf("%d%%", attacker.hero.SpellHitChance(attacker.HeroSheet(), a.monster.Level))
				},
			}
		},
	}
}

// boltReaches: a bolt from `from` flies into `to` without a wall or solid object (a bookcase) stopping it first.
// ponytail: applied to every targeted spell's auto-aim, even ones that don't fly (Teleport); split per spell if that bites.
func boltReaches(m *gridmap.GridMap[*Actor, *Item, Object], from, to geometry.Point) bool {
	for _, p := range (&spellCtx{m: m}).ray(from, to, 20) {
		if p == to {
			return true
		}
		if o := objectAt(m, p); o != nil && !o.IsWalkable(nil) {
			return false
		}
	}
	return false
}

// CastFromMemory casts a memorized spell: D1 checks the spell level and mana, and takes the mana only if it worked.
func (f *Factory) CastFromMemory(caster *Actor, spell int, target geometry.Point) {
	h := caster.hero
	s := caster.HeroSheet()
	if h.SpellLevel(s, spell) <= 0 {
		return
	}
	if f.Generator.D.Spells[spell].Sustained { // a toggle: up until cast again; what it costs is paid as it works
		name := f.Generator.D.Spells[spell].Name
		if caster.HasStatusEffect(StatusEffect(f.Generator.D.Spells[spell].ID)) {
			caster.GetStatusEffects().SetFalse(f.Generator.D.Spells[spell].ID)
			f.Msg(fmt.Sprintf("%s ends.", name))
		} else {
			caster.AddStatusEffect(StatusEffect(f.Generator.D.Spells[spell].ID), -1) // negative: it never ticks down; the shot reads the level
			f.Msg(fmt.Sprintf("%s is up.", name))
		}
		return
	}
	cost := f.Generator.D.ManaCost(h, s, spell)
	mana := caster.GetStats().GetResource(stats.Mana)
	if mana.Value() < rules.Whole(cost) || s.Flags&rules.ISplNoMana != 0 && cost > 0 {
		f.Msg("Not enough mana.")
		f.PlayHeroSound(35)
		return
	}
	if spell == rules.SplIdentify && f.identifyTarget == nil {
		f.pickIdentifyTarget(caster, func() { f.CastFromMemory(caster, spell, target) })
		return
	}
	if f.castSpell(caster, spell, h.SpellLevel(s, spell), target) {
		mana.Decrease(rules.Whole(cost))
		h.ManaBase -= cost
	}
}

// castSpell casts and ends the player's turn; false if the spell did nothing.
func (f *Factory) castSpell(caster *Actor, spell, slvl int, target geometry.Point) bool {
	action := NewSpellAction(caster, spell, slvl, target)
	f.ActionBroker.Execute(action)
	if action.Succeeded() && caster == f.State.GetPlayer() {
		f.ActionBroker.Execute(NewEndPlayerTurnAction(action.Cause(), action.GetDelayInAut()))
	}
	return action.Succeeded()
}

// spellItemAction makes scrolls and staves castable: they cast at the hero's spell level, without mana (UseMana).
func (f *Factory) spellItemAction(item *Item) {
	it := item.roll
	sd := f.Generator.D.Spells[it.Spell]
	var use func(caster *Actor, target geometry.Point)
	use = func(caster *Actor, target geometry.Point) {
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
		if it.MaxDur > 0 && it.Durability == 0 { // broken items do nothing until repaired, like in Hero.Sheet
			f.Msg(fmt.Sprintf("Your %s is broken.", item.Name()))
			refund()
			return
		}
		if s.Mag < it.MinMag {
			f.Msg(fmt.Sprintf("You need %d magic to use that.", it.MinMag))
			refund()
			return
		}
		if it.Spell == rules.SplIdentify && f.identifyTarget == nil {
			refund()
			f.pickIdentifyTarget(caster, func() {
				item.UseCharge()
				if !item.HasChargesLeft() && item.SelfDestructs() {
					caster.GetInventory().Remove(item)
				}
				use(caster, target)
			})
			return
		}
		if !f.castSpell(caster, it.Spell, max(h.SpellLevel(s, it.Spell), it.SLvl), target) {
			refund()
			return
		}
		if it.MaxCharges > 0 {
			it.Charges = max(it.Charges-1, 0)
			item.Refresh()
		}
	}
	desc := []util.LabelText{util.LabelTextFromStringWithColor(f.Generator.D.Spells[it.Spell].Description, defs.OffWhite)}
	if f.Generator.D.Spells[it.Spell].Targeted {
		action := NewTargetedAction(sd.Name, OneTargetTile(f.State.GetMap, util.Arguments{}),
			&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { use(caster, target) },
				Description: desc},
			f.skillTargeting(it.Spell, it.SLvl))
		item.SetTargetedAction(action)
	} else {
		action := NewUntargetedAction(sd.Name, item.icon)
		action.SetEffect(&UntargetedEffect{Effect: func(caster *Actor, _ EffectSource, _ bool) { use(caster, caster.MapPosition()) },
			Description: desc})
		item.SetUntargetedAction(action)
	}
}

// ReadD1Book ports UseItem's IMISC_BOOK: the spell level rises and some mana comes back. The magic requirement
// is checked by the item's UntargetedAction condition (see item.go), so it's guaranteed met here.
func (f *Factory) readBook(user *Actor, it *rules.Item) {
	h := user.hero
	if h == nil {
		return
	}
	h.SpellLevels[it.Spell] = min(h.SpellLevels[it.Spell]+1, 15)
	sd := f.Generator.D.Spells[it.Spell]
	user.GetStats().GetResource(stats.Mana).Increase(float64(sd.Mana))
	f.RefreshSpells(user)
	f.Msg(fmt.Sprintf("You learn %s (level %d).", sd.Name, h.SpellLevels[it.Spell]))
}

// manaShield ports MI_Manashield for a hit of dam whole points: mana pays first, for less (rules.ManaShieldCost).
// raiseManaShield ports AddManashield. MI_Manashield takes a third off only when the shield was cast above spell
// level 0, so the mana_shield status keeps that as its value: -2 with the third, -1 without (neither ticks down).
func raiseManaShield(a *Actor, slvl int) {
	a.AddStatusEffect(StatusManaShield, map[bool]int{true: -2, false: -1}[slvl > 0])
}

func shieldTakesThird(a *Actor) bool {
	return a.statusEffects.AllEntries()[string(StatusManaShield)] == -2
}

func manaShield(def *Actor, dam int) int {
	if !def.HasStatusEffect(StatusManaShield) || dam <= 0 {
		return dam
	}
	if shieldTakesThird(def) {
		dam = rules.ManaShieldCost(dam, 1)
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

// SpellName is a spell's name for menus.
func (s *State) SpellName(spell int) string { return s.factory.Generator.D.Spells[spell].Name }

// LacksMana: the player's memorized spell of that name costs more mana than he has (CastFromMemory's check).
func (s *State) LacksMana(player *Actor, name string) bool {
	h := player.hero
	if h == nil {
		return false
	}
	sheet := player.HeroSheet()
	for spell := range h.SpellLevels {
		if h.SpellLevel(sheet, spell) <= 0 || s.SpellName(spell) != name {
			continue
		}
		cost := s.factory.Generator.D.ManaCost(h, sheet, spell)
		return player.GetStats().GetResource(stats.Mana).Value() < rules.Whole(cost) || sheet.Flags&rules.ISplNoMana != 0 && cost > 0
	}
	return false
}
