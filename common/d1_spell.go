package common

import (
	"fmt"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
	"slices"
	"strconv"
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
	c.setSpell(a.spell)
	from := c.origin() // Teleport moves the caster
	if ok, handled := c.castFromData(a.target); handled {
		a.castSucceeded = ok
	} else {
		a.castSucceeded = c.cast(a.target)
	}
	if a.castSucceeded {
		sfx := d1.SpellSfx[a.spell]
		playD1SfxAt(ctx, sfx[0], from)
		playD1SfxAt(ctx, sfx[1], from)
		ctx.Msg(fmt.Sprintf("You cast %s.", f.Generator.D.Spells[a.spell].Name))
	}
	a.animations = c.animations
	return c.finish()
}

// D1CastAut: casting takes the class's spell animation (decision 13).
func D1CastAut(caster *Actor) int {
	class := 0
	if caster.hero != nil {
		class = caster.hero.Class
	}
	return d1TicksToAUT(d1.SpellFrames[class])
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
	from        *geometry.Point // where the spell comes from when not the caster: a rune
	heroDealt   int             // whole points the hero's own TARGET_BOTH missiles dealt to him
	lastBolt    *ProjectileDirectionAnimation
	// how a hit resolves, set by whatever is dealing the damage rather than read back off a spell id
	resist     int  // d1.ResistKind*
	undeadOnly bool // only the undead are harmed (Holy Bolt)
	drainThird bool // the hit takes a third of what the target has left (Bone Spirit)
}

// setSpell binds the hit rules of the spell being cast: its resistance kind and the two spells whose hits are
// special. Nothing below strike reads the spell id.
func (c *spellCtx) setSpell(spell int) {
	c.spell = spell
	c.resist = d1.SpellResist[spell]
	c.undeadOnly = spell == d1.SplHolyBolt
	c.drainThird = spell == d1.SplBoneSpirit
}

func (c *spellCtx) origin() geometry.Point {
	if c.from != nil {
		return *c.from
	}
	return c.caster.MapPosition()
}

func (c *spellCtx) monsterAt(p geometry.Point) *Actor {
	if a, ok := c.m.TryGetActorAt(p); ok && a.d1Monster != nil && !a.d1Monster.Golem && a.GetStats().GetResource(stats.Health).Value() > c.dealt[a] {
		return a
	}
	return nil
}

// hitCell is monsterAt for a cell a spell hits: a barrel there breaks first.
func (c *spellCtx) hitCell(p geometry.Point) *Actor {
	c.f.hitD1Object(c.m, p, c.caster)
	return c.monsterAt(p)
}

func (c *spellCtx) open(p geometry.Point) bool { return c.m.Contains(p) && c.m.IsTransparent(p) }

// chargedBoltCells lists the cells a charged bolt from o wanders through (d1.ChargedBoltFlight) up to the first wall.
func (c *spellCtx) chargedBoltCells(o, target geometry.Point, rnd int) []geometry.Point {
	var cells []geometry.Point
	p := o
	for _, t := range d1.ChargedBoltFlight(D1DirTo(o, target), rnd) {
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
	o := d1ObjectAt(m, p)
	return o != nil && !o.IsWalkable(nil)
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

// HasInfravision: the spell, the Murky Pool, or an item with the infravision affix (_pInfraFlag).
func (a *Actor) HasInfravision() bool {
	return a.HasStatusEffect(StatusInfravision) || a.hero != nil && a.HeroSheet().Flags&d1.ISplInfravision != 0
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
	resist, immune := resistBits(c.resist)
	if mon.MagicRes&immune != 0 || c.undeadOnly && mon.Data.Class != "MC_UNDEAD" && mon.AI != "AI_DIABLO" {
		return false
	}
	if c.drainThird {
		dam64 = (a.GetStats().GetResource(stats.Health).Value() - c.dealt[a]) << 6 / 3
	}
	chance := c.h.SpellHitChance(c.s, mon.Level)
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
	if _, seen := c.dealt[a]; !seen {
		c.order = append(c.order, a)
	}
	c.dealt[a] += total >> 6
	c.animations = append(c.animations, NewHitAnimation(a.MapPosition(), 0.3, defs.IconEffectHit, 2))
	return true
}

func (c *spellCtx) finish() []Action {
	var actions []Action
	if player := c.f.State.GetPlayer(); c.heroDealt > 0 && player.IsAlive() {
		actions = append(actions, NewChangeResourceAction(c.cause, player, stats.Health, -float64(c.heroDealt)))
	}
	for _, a := range c.order {
		if c.dealt[a] > 0 {
			actions = append(actions, NewChangeResourceAction(c.cause, a, stats.Health, -float64(c.dealt[a])))
		}
	}
	return actions
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
	case d1.ResistKindFire:
		mis = MisFireWall
	case d1.ResistKindLightning:
		mis = MisLightCtrl
	}
	mc := &missileCtx{ctx: c.ctx, from: c.origin(), player: player, c: c}
	mc.hit(mis, p, c.origin(), mind, maxd, shift, ticks)
	c.heroDealt += mc.dealt
}

func (c *spellCtx) secPerTile() float64 {
	return d1.MissileSecondsPerTile(d1.SpellMissileVel(c.spell, c.slvl))
}

func (c *spellCtx) icon() int32 { return d1SpellIcons[c.f.Generator.D.Spells[c.spell].Type] }

// bolt flies from `from` towards `to` until it hits a monster or a wall; a miss flies on (CheckMissileCol).
// It returns where it stopped: the monster's, solid object's or wall's cell (a wall stops a missile on it), else
// the last cell it reached, and whether something stopped it. Barrels in its way break.
func (c *spellCtx) bolt(from, to geometry.Point, hit func(a *Actor) bool) (impact geometry.Point, stopped bool) {
	last := from
	cells, wall, hitWall := c.rayToWall(from, to, 20)
	impact, stopped = wall, hitWall
	for _, p := range cells {
		last = p
		if o := d1ObjectAt(c.m, p); o != nil && !o.IsWalkable(nil) {
			c.f.hitD1Object(c.m, p, c.caster)
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
	if b := c.lastBolt; b != nil && b.End() == from { // Elemental's homing leg flies on from where the first one stopped
		b.Extend(last)
	} else if sfx := d1.SpellSfx[c.spell][2]; last != from {
		c.lastBolt = NewProjectileDirectionAnimation(c.caster, from, last, c.secPerTile(), c.icon(), 2)
		c.lastBolt.SetOnFinish(func() { playD1SfxAt(c.ctx, sfx, last) })
		c.animations = append(c.animations, c.lastBolt)
	} else {
		playD1SfxAt(c.ctx, sfx, last)
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
	declared := d1.SpellEffect[c.spell]
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
	if hp := d1.SpellHealthCost[c.spell]; hp > 0 {
		c.payHP(hp)
	}
	zone := HitZoneFromString(c.f.State.GetMap, d1.SpellHitZone[c.spell])
	positions := zone.GetHitPositions(c.origin(), target)
	for _, effect := range effects {
		effect.Activate(c.caster, target, positions)
	}
	return true, true
}

// castAllowed holds the two checks every cast makes: the spell has to work here, and a spell that needs a victim
// needs one other than the caster.
func (c *spellCtx) castAllowed(target geometry.Point) bool {
	if defs.IsTownLevel(c.m.GetName()) && !c.f.Generator.D.Spells[c.spell].Town {
		c.ctx.Msg("That spell doesn't work in town.")
		c.f.PlayD1HeroSound(27)
		return false
	}
	if d1.SpellTargeted[c.spell] && target == c.origin() && c.spell != d1.SplTeleport && c.spell != d1.SplGolem {
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
	case d1.SplIdentify:
		item := f.identifyTarget
		if item == nil {
			return false
		}
		item.d1.Identified = true
		item.RefreshD1()
		SyncHero(c.caster)
		f.Msg(fmt.Sprintf("It is %s.", item.Name()))
		if f.UI != nil {
			f.UI().ShowItemInfo(item)
		}
	case d1.SplJester: // missiles_jester: one of nine spells at the Jester's spell level
		c.spell = [10]int{d1.SplFirebolt, d1.SplFirebolt, d1.SplFireball, d1.SplFireWall, d1.SplGuardian, d1.SplChain,
			d1.SplTown, d1.SplTeleport, d1.SplApocalypse, d1.SplStone}[d1Rng.Intn(10)]
		c.setSpell(c.spell)
		c.ctx.Msg(fmt.Sprintf("The Jester casts %s.", f.Generator.D.Spells[c.spell].Name))
		return c.cast(target)
	case d1.SplWarp: // missiles_warp: to the nearest stairs, beside them
		return f.d1Warp(c)
	case d1.SplTown:
		return f.openD1Portal(c.caster)
	case d1.SplGuardian:
		p, ok := c.freeNear(target, c.emptyFloor)
		if !ok {
			return false
		}
		f.spawnD1Guardian(c.caster, p, d1.GuardianTurns(clvl, slvl))
	case d1.SplGolem:
		return f.summonD1Golem(c, target)
	case d1.SplTelekinesis:
		return f.d1Telekinesis(c, target)
	default:
		return false
	}
	return true
}

// spawnD1FireWall places one cell of a fire wall (AddFirewall, MI_Firewall), for Fire Wall or Ring of Fire.
// Hellfire's burns the hero too, dlvl·16 ticks longer, and for a flat 8·clvl/64 a tick (AddFirewall's bug).
func (f *Factory) spawnD1FireWall(caster *Actor, p geometry.Point, slvl, layers int) {
	clvl := caster.hero.Level
	turns := d1.FireWallTurns(slvl, d1DungeonLevel(f.State))
	f.spawnD1Wall(caster, p, d1.ResistKindFire, turns, true, layers, defs.IconEffectFireSmall, func() int { return d1.HellfireFireWallDamage64(clvl) })
}

// spawnD1LightningWall places one cell of a lightning wall (MIS_LIGHTWALL): 255·(slvl+1) ticks of 16·(2d10 + clvl
// + 2)/64 a tick, rolled once when it is laid, on monsters and the hero alike.
func (f *Factory) spawnD1LightningWall(caster *Actor, p geometry.Point, slvl, clvl, layers int) {
	dam := 16 * (d1Rng.Intn(10) + d1Rng.Intn(10) + clvl + 2) // one roll a controller tick, shared by what it lays
	f.spawnD1Wall(caster, p, d1.ResistKindLightning, 255*(slvl+1)/d1.RefWalkTicks, true, layers, defs.IconEffectLightning, func() int { return dam })
}

// spawnD1Wall places one wall cell of `layers` walls stacked (a fire wall's start cell holds two), each rolling
// dam64() once as it is laid: each turn every layer hits the monster standing in it, and with both the hero too,
// for its 1/64 a tick over a turn's 8 ticks.
func (f *Factory) spawnD1Wall(caster *Actor, p geometry.Point, resist, turns int, both bool, layers int, icon int32, dam64 func() int) {
	m := f.State.GetMap()
	if m.IsObjectAt(p) || turns <= 0 {
		return
	}
	dams := make([]int, layers)
	for i := range dams {
		dams[i] = dam64()
	}
	wall := NewFire(0)
	wall.icon, wall.turnsUntilRemoval = icon, turns
	wall.SetDealDamage(func(location geometry.Point, _ func() bool) {})
	wall.SetOnTurn(func() {
		if !wall.IsAlive() || f.State.GetMap() != m {
			m.RemoveObject(wall)
			wall.turnsUntilRemoval = 0
			return
		}
		a, ok := m.TryGetActorAt(p)
		if !ok || !caster.IsAlive() || a.d1Monster == nil && !(both && a == f.State.GetPlayer()) {
			return
		}
		f.ActionBroker.Execute(&spellTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			caster: caster, resist: resist, run: func(c *spellCtx) {
				c.from = &p
				for _, d := range dams {
					if v := c.monsterAt(p); v != nil {
						c.strike(v, d, d1.RefWalkTicks)
					} else if both {
						c.hurtHero(p, d, d, true, d1.RefWalkTicks)
					}
				}
			}})
	})
	f.GetTimeKeeper().AddTimed(wall)
	m.AddObject(wall, p)
}

// spawnD1Guardian places a Guardian: every other turn it spits a firebolt at the closest monster it can see (MI_Guardian).
func (f *Factory) spawnD1Guardian(caster *Actor, p geometry.Point, turns int) {
	m := f.State.GetMap()
	g := NewFire(0)
	g.icon, g.frameCount, g.turnsUntilRemoval = defs.IconEffectFireBig, 2, turns
	g.SetDealDamage(func(geometry.Point, func() bool) {})
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
					playD1SfxAt(c.ctx, d1.SpellSfx[d1.SplFirebolt][1], p)
					c.bolt(p, best.MapPosition(), func(a *Actor) bool { return c.strike(a, dam<<6, 1) })
				}
			}})
	})
	f.GetTimeKeeper().AddTimed(g)
	m.AddObject(g, p)
}

// spellTickAction resolves the damage of a lingering spell (fire wall, guardian) on its turn.
type spellTickAction struct {
	BaseAction
	caster     *Actor
	spell      int // only for the runes, which cast their own spell through the content mapping
	resist     int // d1.ResistKind* of the damage this tick deals, when it is not a whole spell's
	run        func(c *spellCtx)
	animations []Animation
}

func (a *spellTickAction) GetAnimations() []Animation { return a.animations }
func (a *spellTickAction) GetDelayInAut() int         { return 0 }
func (a *spellTickAction) Execute(ctx ActionContext) []Action {
	c := &spellCtx{f: ctx.GameState.factory, ctx: ctx, caster: a.caster, h: a.caster.hero, s: a.caster.HeroSheet(), spell: a.spell,
		m: ctx.GameState.GetMap(), cause: a.cause, dealt: map[*Actor]int{}}
	c.setSpell(a.spell)
	if a.resist != 0 {
		c.resist = a.resist
	}
	a.run(c)
	a.animations = c.animations
	return c.finish()
}

// d1MovePlayer puts the player on a cell (Teleport, Phasing, the Holy shrine).
func (f *Factory) d1MovePlayer(player *Actor, p geometry.Point, why ActionName) {
	m := f.State.GetMap()
	m.MoveActor(player, p)
	player.SetDrawPosition(p.ToPointF())
	f.ActionBroker.QueueForExecution(NewUpdateFoVAction(NewActorCause(why, player), player))
}

// d1RandomTeleport ports MIS_RNDTELEPORT: a jump of 4-6 cells along each axis.
func (f *Factory) d1RandomTeleport(player *Actor) {
	m, p := f.State.GetMap(), player.MapPosition()
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

// pickD1IdentifyTarget lists the unidentified items before Identify costs anything; picking one runs cast with it
// as the target, so cancelling the list keeps the scroll or mana (D1's cursor mode can be cancelled too).
func (f *Factory) pickD1IdentifyTarget(player *Actor, cast func()) {
	var items []*Item
	for _, it := range d1HeroItems(player) {
		if it.d1 != nil && it.d1.Magical != 0 && !it.d1.Identified {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		f.Msg("You have nothing to identify.")
		return
	}
	if f.UI != nil {
		openD1ItemList(f.UI(), player, items, func(item *Item) {
			f.identifyTarget = item
			cast()
			f.identifyTarget = nil
		})
	}
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
	f.placeD1Golem(c.m, p, SavedGolem{AC: gs.AC, Hit: gs.Hit, MinDam: gs.MinDam, MaxDam: gs.MaxDam, MaxHP: gs.HP, HP: gs.HP})
	return true
}

// SavedGolem is the player's golem, which follows them from level to level and into the save.
type SavedGolem struct {
	AC, Hit, MinDam, MaxDam, MaxHP, HP int
}

func (f *Factory) placeD1Golem(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point, g SavedGolem) *Actor {
	a := f.newD1Monster(f.Generator.D.MonsterByID("MT_GOLEM"), nil) // the player's own: no Nightmare scaling
	a.isHostile = false
	mon := a.d1Monster
	mon.Golem, mon.SpawnIndex, mon.AC, mon.Hit, mon.MinDam, mon.MaxDam = true, -1, g.AC, g.Hit, g.MinDam, g.MaxDam
	a.SetColor(defs.White)
	a.GetStats().SetAttribute(stats.MaxHealth, g.MaxHP)
	SetResource(a.GetStats().GetResource(stats.Health), max(g.HP, 1))
	a.SetDrawPosition(p.ToPointF())
	m.AddActor(a, p)
	f.State.OnActorSpawned(a)
	return a
}

// D1Golem is the player's living golem on m, if any.
func D1Golem(m *gridmap.GridMap[*Actor, *Item, Object]) *Actor {
	for _, a := range m.Actors() {
		if a.d1Monster != nil && a.d1Monster.Golem && a.IsAlive() {
			return a
		}
	}
	return nil
}

// SaveD1Golem describes a golem for moving or saving it.
func SaveD1Golem(a *Actor) SavedGolem {
	mon, st := a.d1Monster, a.GetStats()
	return SavedGolem{AC: mon.AC, Hit: mon.Hit, MinDam: mon.MinDam, MaxDam: mon.MaxDam, MaxHP: st.GetAttribute(stats.MaxHealth), HP: st.GetResource(stats.Health).Value()}
}

// PlaceD1GolemNear puts the player's golem back next to the player (after a level change or loading a save).
func (f *Factory) PlaceD1GolemNear(player *Actor, g SavedGolem) {
	m := f.State.GetMap()
	if cells := freeCellsNear(m, player.MapPosition(), 1, m.CanPlaceActorHere); len(cells) > 0 {
		f.placeD1Golem(m, cells[0], g)
	}
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

func (o *d1PortalObject) Name() string                    { return "Town Portal" }
func (o *d1PortalObject) MapPosition() geometry.Point     { return o.pos }
func (o *d1PortalObject) SetMapPosition(p geometry.Point) { o.pos = p }
func (o *d1PortalObject) Icon(tick uint64) int32 {
	return []int32{defs.Icon("prop.portal"), defs.Icon("prop.portal.2")}[util.GetLoopingFrameFromTick(tick, 0.3, 2)]
}
func (o *d1PortalObject) IsWalkable(*Actor) bool              { return true }
func (o *d1PortalObject) IsTransparent() bool                 { return true }
func (o *d1PortalObject) IsPassableForProjectile() bool       { return true }
func (o *d1PortalObject) Turn()                               {}
func (o *d1PortalObject) ActorWalkedOn(*Actor, func() bool)   {}
func (o *d1PortalObject) ActorBumpedInto(*Actor, func() bool) {}

func (s *State) D1Portal() *D1Portal     { return s.d1Portal }
func (s *State) SetD1Portal(p *D1Portal) { s.d1Portal = p }

func (f *Factory) openD1Portal(player *Actor) bool {
	m := f.State.GetMap()
	if old := f.State.d1Portal; old != nil {
		f.State.CloseD1Portal()
	}
	cells := freeCellsNear(m, player.MapPosition(), 1, func(p geometry.Point) bool {
		_, isTransition := m.GetTransitionAt(p)
		return m.CanPlaceActorHere(p) && !m.IsObjectAt(p) && !isTransition
	})
	if len(cells) == 0 {
		return false
	}
	f.State.d1Portal = &D1Portal{Map: m.GetName(), X: cells[0].X, Y: cells[0].Y}
	PlaceD1Portal(f.State, m)
	return true
}

// PlaceD1Portal puts the town portal's two ends on a map when it has one: at the dungeon spot, or at the town's
// "portal" Location in maps.ldtk.
func PlaceD1Portal(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	p := s.d1Portal
	if p == nil {
		return
	}
	var pos geometry.Point
	var target gridmap.Transition
	switch {
	case m.GetName() == p.Map:
		pos, target = geometry.Point{X: p.X, Y: p.Y}, gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: d1PortalLocation}
	case m.GetName() == string(defs.LevelNameTown):
		pos, target = m.GetNamedLocation(d1PortalLocation), gridmap.Transition{TargetMap: p.Map, TargetLocation: d1PortalLocation}
	default:
		return
	}
	if m.IsObjectAt(pos) {
		return
	}
	m.AddNamedLocation(d1PortalLocation, pos)
	m.AddObject(&d1PortalObject{pos: pos, target: target}, pos)
	m.AddTransitionAt(pos, target)
	delete(s.glowCache, m) // the portal is a light source
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
				delete(s.glowCache, m)
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
	if t, u := f.d1ClassSkill(player); t != nil {
		t.SetInternalName(d1ClassSkillID)
		targeted = append(targeted, t)
	} else if u != nil {
		u.SetInternalName(d1ClassSkillID)
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
		desc := d1SpellTooltip(d1.SpellDescriptions[sp], d1.SpellDamageText(sp, s.Mag, h.Level, slvl), slvl, f.Generator.D.ManaCost(h, s, spell)>>6)
		if d1.SpellTargeted[spell] {
			action := NewTargetedAction(sd.Name, OneTargetTile(f.State.GetMap, util.Arguments{}),
				&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { f.CastD1FromMemory(caster, sp, target) },
					Description: desc},
				d1SpellTargeting(f, sp))
			action.SetIcon(defs.Icon(defs.IconID("spell", sd.ID)))
			action.SetInternalName(sd.ID)
			targeted = append(targeted, action)
		} else {
			action := NewUntargetedAction(sd.Name, defs.Icon(defs.IconID("spell", sd.ID)))
			action.SetEffect(&UntargetedEffect{Effect: func(caster *Actor, _ EffectSource) { f.CastD1FromMemory(caster, sp, caster.MapPosition()) },
				Description: desc})
			action.SetInternalName(sd.ID)
			untargeted = append(untargeted, action)
		}
	}
	return targeted, untargeted
}

// d1SpellTooltip is a memorized spell's popup: what it does, its damage, then its spell level and what a cast costs.
func d1SpellTooltip(text, damage string, slvl, mana int) []util.LabelText {
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

// d1ClassSkillID is the class skill's internal name: a hero has exactly one.
const d1ClassSkillID = "CLASS_SKILL"

// RefreshD1Spells updates the player's spell skills after learning or changing gear. A skill the player already
// had keeps its pointer (updated in place), so the belt and anything else holding it stays current.
func (f *Factory) RefreshD1Spells(player *Actor) {
	t, u := f.D1SpellSkills(player)
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

func d1SpellTargeting(f *Factory, spell int) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, aoe *AoEResolver, m *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &RangedTargetingRules{
				attacker: attacker,
				isOnMap:  m.Contains,
				targets: func() []geometry.Point {
					return targetsByDistance(m, attacker.MapPosition(), func(a *Actor) bool {
						return a.IsVisible() && a.IsHostileTo(attacker) && attacker.CanSee(a.MapPosition()) && !a.IsVendor() &&
							d1BoltReaches(m, attacker.MapPosition(), a.MapPosition())
					})
				},
				aoe: aoe,
				infoForTarget: func(p geometry.Point) string {
					a, ok := m.TryGetActorAt(p)
					if !ok || a.d1Monster == nil || attacker.hero == nil {
						return ""
					}
					return fmt.Sprintf("%d%%", attacker.hero.SpellHitChance(attacker.HeroSheet(), a.d1Monster.Level))
				},
			}
		},
	}
}

// d1BoltReaches: a bolt from `from` flies into `to` without a wall or solid object (a bookcase) stopping it first.
// ponytail: applied to every targeted spell's auto-aim, even ones that don't fly (Teleport); split per spell if that bites.
func d1BoltReaches(m *gridmap.GridMap[*Actor, *Item, Object], from, to geometry.Point) bool {
	for _, p := range (&spellCtx{m: m}).ray(from, to, 20) {
		if p == to {
			return true
		}
		if o := d1ObjectAt(m, p); o != nil && !o.IsWalkable(nil) {
			return false
		}
	}
	return false
}

// CastD1FromMemory casts a memorized spell: D1 checks the spell level and mana, and takes the mana only if it worked.
func (f *Factory) CastD1FromMemory(caster *Actor, spell int, target geometry.Point) {
	h := caster.hero
	s := caster.HeroSheet()
	if h.SpellLevel(s, spell) <= 0 {
		return
	}
	cost := f.Generator.D.ManaCost(h, s, spell)
	mana := caster.GetStats().GetResource(stats.Mana)
	if mana.Value() < cost>>6 || s.Flags&d1.ISplNoMana != 0 && cost > 0 {
		f.Msg("Not enough mana.")
		f.PlayD1HeroSound(35)
		return
	}
	if spell == d1.SplIdentify && f.identifyTarget == nil {
		f.pickD1IdentifyTarget(caster, func() { f.CastD1FromMemory(caster, spell, target) })
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
	f.ActionBroker.Execute(action)
	if action.Succeeded() && caster == f.State.GetPlayer() {
		f.ActionBroker.Execute(NewEndPlayerTurnAction(action.Cause(), action.GetDelayInAut()))
	}
	return action.Succeeded()
}

// d1SpellItemAction makes scrolls and staves castable: they cast at the hero's spell level, without mana (UseMana).
func (f *Factory) d1SpellItemAction(item *Item) {
	it := item.d1
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
		if it.Spell == d1.SplIdentify && f.identifyTarget == nil {
			refund()
			f.pickD1IdentifyTarget(caster, func() {
				item.UseCharge()
				if !item.HasChargesLeft() && item.SelfDestructs() {
					caster.GetInventory().Remove(item)
				}
				use(caster, target)
			})
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
	desc := []util.LabelText{util.LabelTextFromStringWithColor(d1.SpellDescriptions[it.Spell], defs.OffWhite)}
	if d1.SpellTargeted[it.Spell] {
		action := NewTargetedAction(sd.Name, OneTargetTile(f.State.GetMap, util.Arguments{}),
			&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { use(caster, target) },
				Description: desc},
			d1SpellTargeting(f, it.Spell))
		item.SetTargetedAction(action)
	} else {
		action := NewUntargetedAction(sd.Name, item.icon)
		action.SetEffect(&UntargetedEffect{Effect: func(caster *Actor, _ EffectSource) { use(caster, caster.MapPosition()) },
			Description: desc})
		item.SetUntargetedAction(action)
	}
}

// ReadD1Book ports UseItem's IMISC_BOOK: the spell level rises and some mana comes back. The magic requirement
// is checked by the item's UntargetedAction condition (see d1_item.go), so it's guaranteed met here.
func (f *Factory) readD1Book(user *Actor, it *d1.Item) {
	h := user.hero
	if h == nil {
		return
	}
	h.SpellLevels[it.Spell] = min(h.SpellLevels[it.Spell]+1, 15)
	sd := f.Generator.D.Spells[it.Spell]
	user.GetStats().GetResource(stats.Mana).Increase(float64(sd.Mana))
	f.RefreshD1Spells(user)
	f.Msg(fmt.Sprintf("You learn %s (level %d).", sd.Name, h.SpellLevels[it.Spell]))
}

// d1ManaShield ports MI_Manashield for a hit of dam whole points: mana pays first, for less (d1.ManaShieldCost).
func d1ManaShield(def *Actor, dam int) int {
	if !def.HasStatusEffect(StatusManaShield) || dam <= 0 {
		return dam
	}
	dam = d1.ManaShieldCost(dam, def.hero.SpellLevel(def.HeroSheet(), d1.SplManaShield))
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

// D1SpellName is a spell's name for menus.
func (s *State) D1SpellName(spell int) string { return s.factory.Generator.D.Spells[spell].Name }

// D1LacksMana: the player's memorized spell of that name costs more mana than he has (CastD1FromMemory's check).
func (s *State) D1LacksMana(player *Actor, name string) bool {
	h := player.hero
	if h == nil {
		return false
	}
	sheet := player.HeroSheet()
	for spell := range h.SpellLevels {
		if h.SpellLevel(sheet, spell) <= 0 || s.D1SpellName(spell) != name {
			continue
		}
		cost := s.factory.Generator.D.ManaCost(h, sheet, spell)
		return player.GetStats().GetResource(stats.Mana).Value() < cost>>6 || sheet.Flags&d1.ISplNoMana != 0 && cost > 0
	}
	return false
}
