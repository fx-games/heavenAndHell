package common

import (
	"fmt"
	"rpg/src/d1"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
)

// d1MagblosTicks is how long MI_LArrow's fire burst (MFILE_MAGBLOS, 10 frames) checks for a hit: _miAnimLen - 1.
const d1MagblosTicks = 9

// TrapMissileAction resolves one trap's missile at once, like the monster missiles: AddMissile with no source
// (id -1), so its damage comes from the dungeon level and PlayerMHit rolls without a monster.
type TrapMissileAction struct {
	BaseAction
	from, target geometry.Point
	mis          D1Missile
}

func NewTrapMissileAction(from geometry.Point, mis D1Missile, target geometry.Point) *TrapMissileAction {
	return &TrapMissileAction{BaseAction: BaseAction{cause: AbstractCause("a trap"), name: ActionNameMonsterMissile},
		from: from, target: target, mis: mis}
}

func (a *TrapMissileAction) Execute(ctx ActionContext) []Action {
	player := ctx.GameState.GetPlayer()
	c := &missileCtx{ctx: ctx, from: a.from, player: player, trap: true, c: &spellCtx{f: ctx.GameState.factory, m: ctx.GameState.GetMap(), dealt: map[*Actor]int{}}}
	cl := ctx.GameState.CurrentLevel().Depth
	playD1SfxAt(ctx, d1MissileSfx[a.mis][0], a.from)
	switch a.mis {
	case MisArrow: // MI_Arrow: currlevel..2·currlevel
		c.bolt(a.mis, a.target, cl, 2*cl)
	case MisFarrow: // MI_LArrow: the arrow hits as a plain one, then bursts in fire where it stopped
		mind, maxd := rng.Intn(10)+1+cl, rng.Intn(10)+1+2*cl
		end := a.from
		for _, p := range c.c.ray(a.from, a.target, 20) {
			end = p
			if c.hitAs(a.mis, p, a.from, mind, maxd, false, 1, d1.ResistNone) {
				break
			}
		}
		// the burst: MAGBLOS runs _miAnimLen-1 = 9 ticks, each a fresh roll that may hit again (nodel)
		for tick := 0; tick < d1MagblosTicks; tick++ {
			mind, maxd = rng.Intn(10)+1+cl, rng.Intn(10)+1+2*cl
			c.hit(a.mis, end, a.from, mind, maxd, false, 1)
		}
		c.c.missile(nil, []geometry.Point{a.from, end}, missileSecPerTile(a.mis), defs.IconEffectFireSmall)
		c.c.flash(end, 0.4, defs.IconEffectFireBig)
	case MisFirebolt: // MI_Firebolt without a source: currlevel + rnd(2·currlevel)
		d := cl + rng.Intn(2*cl)
		c.bolt(a.mis, a.target, d, d)
	case MisLightCtrl: // MI_Lightctrl: a lightning (8 ticks, in 1/64) on every cell of the line
		dam := rng.Intn(cl) + 2*cl
		cells := c.c.ray(a.from, a.target, 20)
		for _, p := range cells {
			if c.hit(a.mis, p, a.from, dam, dam, true, 8) {
				break
			}
		}
		if len(cells) > 0 {
			c.c.missile(nil, []geometry.Point{a.from, cells[len(cells)-1]}, missileSecPerTile(a.mis), defs.IconEffectLightning)
		}
	case MisFireRing: // mi_fire_ring: fire walls on the ring three cells out that the chest sees, until one lands on
		// a piece that stops missiles; each burns (10 + currlevel)·16 ticks at currlevel·8/64 a tick
		if lvl := ctx.GameState.mapGen.d1Levels[c.c.m.GetName()]; lvl != nil {
			for _, p := range d1Ring(a.from, 3) {
				if !c.c.m.Contains(p) || lvl.CellSolid(p.X, p.Y) || c.c.m.IsObjectAt(p) || !c.c.lineClear(a.from, p) {
					continue
				}
				if lvl.CellMissile(p.X, p.Y) {
					break
				}
				c.hazard(MisFireRing, p, (10+cl)*16/d1.RefWalkTicks, 8*cl)
			}
		}
	case MisFireWall: // AddFirewall from the shrine at the hero: 8/64 a tick per hero level while it burns through
		// ponytail: one pass over the hero (two ticks), not the travelling wall D1 lets run on for (10+currlevel)·16 ticks
		lvl := 1
		if player.hero != nil {
			lvl = player.hero.Level
		}
		if p := player.MapPosition(); c.c.lineClear(a.from, p) {
			c.hit(a.mis, p, a.from, 8*lvl, 8*lvl, true, 2)
		}
		c.c.missile(nil, []geometry.Point{a.from, player.MapPosition()}, missileSecPerTile(a.mis), defs.IconEffectFireBig)
	case MisFlashTrap: // AddFlash for the players: monster[0]'s level (the golem slot) ·2, on the six cells below
		dam := 2 * ctx.GameState.factory.Generator.D.Monsters[d1.MtGOLEM].Level
		for _, d := range []geometry.Point{{X: -1}, {}, {X: 1}, {X: -1, Y: 1}, {Y: 1}, {X: 1, Y: 1}} {
			c.hit(a.mis, a.from.Add(d), a.from, dam, dam, true, d1.FlashTicks)
		}
		c.c.flash(a.from, 0.4, defs.IconEffectLightning)
	case MisNova: // AddNova without a source: currlevel/2 + 3·rnd(3). MI_Nova sends a lightning ball towards each of
		// NovaRays' offsets; each flies until it hits something, so the hero can be struck by every ball crossing its cell
		dam := cl>>1 + rng.Intn(3) + rng.Intn(3) + rng.Intn(3)
		for _, r := range d1.NovaRays {
			c.fly(a.mis, c.c.ray(a.from, a.from.Add(geometry.Point{X: r[0], Y: r[1]}), 20), dam, dam, false)
		}
	}
	c.c.emit(ctx.ActionBroker)
	switch {
	case c.taken > 0:
		ctx.Msg(fmt.Sprintf("The trap hits you for %d damage.", max(c.taken>>6, 1)))
	case c.block:
		ctx.Msg("You block the trap.")
	default:
		ctx.Msg("The trap misses you.")
	}
	c.c.cause = a.cause
	hits := c.c.finish() // the monsters it hit
	if c.dealt <= 0 || !player.IsAlive() {
		return hits
	}
	return append(hits, NewChangeResourceAction(a.cause, player, stats.Health, -float64(c.dealt)))
}

// d1TrapTargets are the objects a wall trap watches (Obj_Trap): doors fire it when opened, the rest when used.
var d1TrapTargets = map[int]bool{d1.ObjL1LDOOR: true, d1.ObjL1RDOOR: true, d1.ObjL2LDOOR: true, d1.ObjL2RDOOR: true,
	d1.ObjL3LDOOR: true, d1.ObjL3RDOOR: true, d1.ObjLEVER: true, d1.ObjCHEST1: true, d1.ObjCHEST2: true,
	d1.ObjCHEST3: true, d1.ObjSWITCHSKL: true, d1.ObjSARC: true}

// springD1WallTraps ports Obj_Trap for the object at piece (px, py), just opened or used: every wall trap aimed at
// it that hasn't fired shoots its missile at the hero standing next to the object, or at the object itself.
func (f *Factory) springD1WallTraps(m *gridmap.GridMap[*Actor, *Item, Object], px, py int) {
	lvl := f.State.mapGen.d1Levels[m.GetName()]
	if lvl == nil || f.State.GetMap() != m {
		return
	}
	var target *d1.Object
	for _, o := range lvl.Objects {
		if o.X == px && o.Y == py && d1TrapTargets[o.Kind] {
			target = o
		}
	}
	if target == nil {
		return
	}
	for _, t := range lvl.Objects {
		if t.Kind != d1.ObjTRAPL && t.Kind != d1.ObjTRAPR || t.Var4 != 0 || t.Var1 != px || t.Var2 != py {
			continue
		}
		t.Var4 = 1
		to := toCell(px, py)
		if p := f.State.GetPlayer().MapPosition(); geometry.DistanceChebyshev(p, to) <= 1 {
			to = p
		}
		mis := map[int]D1Missile{0: MisArrow, 1: MisFirebolt, 2: MisLightCtrl}[t.Var3]
		f.Msg("A trap fires from the wall!")
		f.PlayD1Sfx("IS_TRAP", toCell(t.X, t.Y)) // Obj_Trap
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(toCell(t.X, t.Y), mis, to))
		target.TrapFlag = false
	}
}

// springD1ChestTrap ports OperateChest's trap: a trapped chest shoots an arrow, a fire arrow or a nova at the hero,
// and Hellfire's also light a ring of fire, steal potions or drain mana.
func (f *Factory) springD1ChestTrap(o *D1Object, player *Actor) {
	obj := o.O
	if !obj.TrapFlag || obj.Kind < d1.ObjTCHEST1 || obj.Kind > d1.ObjTCHEST3 {
		return
	}
	obj.TrapFlag = false
	f.Msg("The chest was trapped!")
	switch obj.Var4 {
	case 0, 1, 2, 3:
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(o.pos, [4]D1Missile{MisArrow, MisFarrow, MisNova, MisFireRing}[obj.Var4], player.MapPosition()))
	case 4:
		f.d1StealPotions(player)
	case 5: // missiles_mana_trap
		if h := player.hero; h != nil {
			h.ManaBase = h.MaxManaBase - player.HeroSheet().MaxMana
			player.GetStats().GetResource(stats.Mana).SetToMin()
			SyncHero(player)
		}
	}
}

// d1StealPotions ports missiles_steal_pots on the pack (H&H has no belt): each potion has an even chance to be
// taken or swapped for a lesser one.
func (f *Factory) d1StealPotions(player *Actor) {
	gen := f.Generator
	byMisc := func(misc string) int {
		for idx, b := range gen.D.Items {
			if b.MiscID == misc && b.Drop != "IDROP_NEVER" {
				return idx
			}
		}
		return -1
	}
	inv := player.GetInventory()
	for _, it := range inv.Items() {
		if it.d1 == nil || it.d1.Type != d1.ITypeMisc || rng.Intn(2) == 0 {
			continue
		}
		into := ""
		switch it.d1.MiscID {
		case "IMISC_HEAL", "IMISC_MANA":
			inv.Remove(it)
			continue
		case "IMISC_FULLHEAL", "IMISC_REJUV": // D1 rolls mana or healing for a rejuvenation, then takes healing anyway
			into = "IMISC_HEAL"
		case "IMISC_FULLMANA":
			into = "IMISC_MANA"
		case "IMISC_FULLREJUV":
			into = [3]string{"IMISC_FULLMANA", "IMISC_FULLHEAL", "IMISC_REJUV"}[rng.Intn(3)]
		default:
			continue
		}
		if it.d1.MiscID == "IMISC_REJUV" {
			rng.Intn(2)
		}
		if idx := byMisc(into); idx >= 0 {
			inv.Remove(it)
			inv.Add(f.NewD1Item(gen.PlainItem(idx)))
		}
	}
}

// d1Ring is the CrawlTable ring at radius r around p: the cells at Chebyshev distance r.
func d1Ring(p geometry.Point, r int) (ring []geometry.Point) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if max(geometry.Abs(dx), geometry.Abs(dy)) == r {
				ring = append(ring, p.Add(geometry.Point{X: dx, Y: dy}))
			}
		}
	}
	return
}

// d1SprungTraps lists the wall traps of a level that fired or were disarmed (_oVar4 set), by object index.
func d1SprungTraps(lvl *d1.Level) (out []int) {
	for _, t := range lvl.Objects {
		if (t.Kind == d1.ObjTRAPL || t.Kind == d1.ObjTRAPR) && t.Var4 != 0 {
			out = append(out, t.Index)
		}
	}
	return
}

// D1BurningCross ports Obj_BCrossDamage for the hero's last turn: standing just north of a burning cross burns
// 6/8/10/12 64ths of a HP per tick by level type, less fire resistance.
func D1BurningCross(s *State, player *Actor, autUsed int) {
	m := s.GetMap()
	o := d1ObjectAt(m, player.MapPosition().Add(geometry.Point{Y: 1}))
	lvl := s.mapGen.d1Levels[m.GetName()]
	if o == nil || o.O.Kind != d1.ObjBCROSS && o.O.Kind != d1.ObjTBCROSS || lvl == nil || player.hero == nil || !player.IsAlive() {
		return
	}
	dam := [4]int{6, 8, 10, 12}[lvl.Type-1]
	if fr := player.HeroSheet().FR; fr > 0 {
		dam -= fr * dam / 100
	}
	ticks := autUsed * d1.RefWalkTicks / 10
	s.factory.ActionBroker.Execute(NewChangeResourceAction(AbstractCause("a burning cross"), player, stats.Health, -float64(dam*ticks)/64))
}
