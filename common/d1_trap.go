package common

import (
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
)

// TrapMissileAction resolves one trap's missile at once, like the monster missiles: AddMissile with no source
// (id -1), so its damage comes from the dungeon level and PlayerMHit rolls without a monster.
type TrapMissileAction struct {
	BaseAction
	from, target geometry.Point
	mis          D1Missile
	animations   []Animation
}

func NewTrapMissileAction(from geometry.Point, mis D1Missile, target geometry.Point) *TrapMissileAction {
	return &TrapMissileAction{BaseAction: BaseAction{cause: AbstractCause("a trap"), name: ActionNameMonsterMissile},
		from: from, target: target, mis: mis}
}

func (a *TrapMissileAction) GetAnimations() []Animation { return a.animations }

func (a *TrapMissileAction) Execute(ctx ActionContext) []Action {
	player := ctx.GameState.GetPlayer()
	c := &missileCtx{ctx: ctx, from: a.from, player: player, c: &spellCtx{m: ctx.GameState.GetMap()}}
	cl := d1DungeonLevel(ctx.GameState)
	switch a.mis {
	case MisArrow: // MI_Arrow: currlevel..2·currlevel
		c.bolt(a.mis, a.target, cl, 2*cl)
	case MisFarrow: // MI_LArrow: the arrow hits as a plain one, then bursts in fire where it stopped
		mind, maxd := d1Rng.Intn(10)+1+cl, d1Rng.Intn(10)+1+2*cl
		end := a.from
		for _, p := range c.c.ray(a.from, a.target, 20) {
			end = p
			if c.hitAs(a.mis, p, a.from, mind, maxd, false, 1, d1.ResistNone) {
				break
			}
		}
		// ponytail: the burst hits once; D1's MAGBLOS animation may roll again on later frames
		mind, maxd = d1Rng.Intn(10)+1+cl, d1Rng.Intn(10)+1+2*cl
		c.hit(a.mis, end, a.from, mind, maxd, false, 1)
		c.c.animations = append(c.c.animations, NewProjectileDirectionAnimation(nil, a.from, end, 0.03, defs.IconEffectFireSmall, 2),
			NewHitAnimation(end, 0.4, defs.IconEffectFireBig, 2))
	case MisFirebolt: // MI_Firebolt without a source: currlevel + rnd(2·currlevel)
		d := cl + d1Rng.Intn(2*cl)
		c.bolt(a.mis, a.target, d, d)
	case MisLightCtrl: // MI_Lightctrl: a lightning (8 ticks, in 1/64) on every cell of the line
		dam := d1Rng.Intn(cl) + 2*cl
		cells := c.c.ray(a.from, a.target, 20)
		for _, p := range cells {
			if c.hit(a.mis, p, a.from, dam, dam, true, 8) {
				break
			}
		}
		if len(cells) > 0 {
			c.c.animations = append(c.c.animations, NewProjectileDirectionAnimation(nil, a.from, cells[len(cells)-1], 0.01, defs.IconEffectLightning, 2))
		}
	case MisNova: // AddNova without a source: currlevel/2 + 3·rnd(3), lightning balls in every direction
		dam := cl>>1 + d1Rng.Intn(3) + d1Rng.Intn(3) + d1Rng.Intn(3)
		// ponytail: one hit on the hero when in line within 7 cells, not D1's 92 balls crossing its cell
		if p := player.MapPosition(); geometry.DistanceChebyshev(a.from, p) <= 7 && c.c.lineClear(a.from, p) {
			c.hit(a.mis, p, a.from, dam, dam, false, 1)
		}
		c.c.animations = append(c.c.animations, NewHitAnimation(a.from, 0.4, defs.IconEffectLightning, 2))
	}
	a.animations = c.c.animations
	if c.dealt <= 0 || !player.IsAlive() {
		return nil
	}
	return OneAction(NewChangeResourceAction(a.cause, player, stats.Health, -float64(c.dealt)))
}

// d1TrapTargets are the objects a wall trap watches (Obj_Trap): doors fire it when opened, the rest when used.
var d1TrapTargets = map[int]bool{d1.ObjL1LDOOR: true, d1.ObjL1RDOOR: true, d1.ObjL2LDOOR: true, d1.ObjL2RDOOR: true,
	d1.ObjL3LDOOR: true, d1.ObjL3RDOOR: true, d1.ObjLEVER: true, d1.ObjCHEST1: true, d1.ObjCHEST2: true,
	d1.ObjCHEST3: true, d1.ObjSWITCHSKL: true, d1.ObjSARC: true}

// springD1WallTraps ports Obj_Trap for the object at piece (px, py), just opened or used: every wall trap aimed at
// it that hasn't fired shoots its missile at the hero standing next to the object, or at the object itself.
func (f *Factory) springD1WallTraps(m *gridmap.GridMap[*Actor, *Item, Object], px, py int) {
	lvl := f.State.mapGen.levels[m.GetName()]
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
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(toCell(t.X, t.Y), mis, to))
		target.TrapFlag = false
	}
}

// springD1ChestTrap ports OperateChest's trap: a trapped chest shoots an arrow, a fire arrow or a nova at the hero.
func (f *Factory) springD1ChestTrap(o *D1Object, player *Actor) {
	obj := o.O
	if !obj.TrapFlag || obj.Kind < d1.ObjTCHEST1 || obj.Kind > d1.ObjTCHEST3 {
		return
	}
	obj.TrapFlag = false
	f.Msg("The chest was trapped!")
	f.ActionBroker.QueueForExecution(NewTrapMissileAction(o.pos, [3]D1Missile{MisArrow, MisFarrow, MisNova}[obj.Var4], player.MapPosition()))
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
	lvl := s.mapGen.levels[m.GetName()]
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
