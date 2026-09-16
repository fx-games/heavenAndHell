package game

import (
	"rpg/common"
	"rpg/d1"
	"rpg/geometry"
	"rpg/stats"
)

// d1Act is one chosen monster action and its time cost in AUT.
type d1Act struct {
	cost int
	mode int
	run  func()
}

// d1Brain runs D1's monster AI (monster.cpp MAI_*) on top of H&H's energy system.
type d1Brain struct {
	g      *GameInstance
	a      *common.Actor
	mon    *common.D1Monster
	pos    geometry.Point
	enemy  geometry.Point
	target *common.Actor // who stands at enemy: the player or their golem
	dx, dy int           // monster minus enemy, D1's mx/my
	fights *bool         // a talking boss' decision this turn (talker)
}

var noAct = d1Act{}

// getD1MonsterAction ports ProcessMonsters for one monster. D1 re-runs the AI every tick while a monster stands,
// so a turn tries up to RefWalkTicks ticks before the monster idles.
func (g *GameInstance) getD1MonsterAction(a *common.Actor) (int, func()) {
	cost, run := g.d1MonsterAction(a)
	if run == nil {
		return cost, nil
	}
	return cost, func() {
		d1MonsterRegen(a, cost)
		run()
	}
}

// d1MonsterRegen ports ProcessMonsters' healing for the time an action takes: mLevel/2 (mLevel on level 1)
// in 1/64 HP per tick, unless a no-heal weapon hit the monster.
func d1MonsterRegen(a *common.Actor, aut int) {
	mon, hp := a.D1Monster(), a.GetStats().GetResource(stats.Health)
	if mon.NoHeal || hp.Value() <= 0 || hp.IsAtMax() {
		return
	}
	perTick := mon.Level >> 1
	if mon.Level <= 1 {
		perTick = mon.Level
	}
	hp.IncreaseByFloat(float64(perTick*aut*d1.RefWalkTicks) / 10 / 64)
}

func (g *GameInstance) d1MonsterAction(a *common.Actor) (int, func()) {
	b := &d1Brain{g: g, a: a, mon: a.D1Monster(), pos: a.MapPosition()}
	if a.GetEnergy() < min(b.walkCost(), b.attackCost()) {
		return 0, nil
	}
	player := g.state.GetPlayer()
	if !player.IsAlive() || a.HasStatusEffect(common.StatusStunned) || a.HasStatusEffect(common.StatusStone) {
		return 10, func() {}
	}
	mon := b.mon
	if mon.Golem || mon.Berserk {
		return b.golem()
	}
	b.enemy, b.target = player.MapPosition(), player
	if golem := g.golemOnMap(b.pos); golem != nil && geometry.DistanceSquared(b.pos, golem.MapPosition()) < geometry.DistanceSquared(b.pos, b.enemy) {
		b.enemy, b.target = golem.MapPosition(), golem // M_Enemy: the closest of player, golem and berserk monsters
	}
	b.dx, b.dy = b.pos.X-b.enemy.X, b.pos.Y-b.enemy.Y
	if player.CanSee(b.pos) {
		if mon.Squelch == 0 {
			g.factory.D1MonsterGreeting(a)
		}
		mon.Squelch, mon.Last = 255, b.enemy
	} else {
		mon.Squelch = max(mon.Squelch-d1.RefWalkTicks, 0)
	}

	act, ok := noAct, false
	for tick := 0; tick < d1.RefWalkTicks && !ok; tick++ {
		if mon.Data.Flags&d1.MFlagSearch != 0 {
			act, ok = b.path()
		}
		if !ok {
			act, ok = b.think()
		}
		if ok {
			act.cost += tick * 10 / d1.RefWalkTicks
		} else {
			mon.StandTicks++
		}
	}
	if !ok {
		return 10, func() {}
	}
	return act.cost, func() {
		act.run()
		mon.LastMode, mon.StandTicks = act.mode, 0
	}
}

func (b *d1Brain) think() (d1Act, bool) {
	mon := b.mon
	switch mon.AI {
	case "AI_ZOMBIE":
		return b.zombie()
	case "AI_FALLEN":
		return b.fallen()
	case "AI_SCAV":
		return b.scav()
	case "AI_SKELBOW":
		return b.skelBow()
	case "AI_SNEAK":
		return b.sneak()
	case "AI_GOATMC":
		return b.round(true)
	case "AI_BAT":
		return b.bat()
	case "AI_GOATBOW":
		return b.ranged(b.shoot)
	case "AI_SUCC":
		return b.ranged(func() (d1Act, bool) { return b.missile(common.MisFlare, 0, common.D1ModeRAttack) })
	case "AI_ACIDUNIQ":
		return b.ranged(func() (d1Act, bool) { return b.missile(common.MisAcid, 0, common.D1ModeRSpAttack) })
	// Hellfire's casters are all MAI_Ranged with a differently coloured flare (MONSTER.CPP); this engine draws
	// one flare, so they share MisFlare and differ only in the damage their own stats give it.
	case "AI_LICH", "AI_ARCHLICH", "AI_PSYCHORB", "AI_NECROMORB":
		return b.ranged(func() (d1Act, bool) { return b.missile(common.MisFlare, 0, common.D1ModeRAttack) })
	case "AI_FIREBAT":
		return b.ranged(func() (d1Act, bool) { return b.missile(common.MisFirebolt, 0, common.D1ModeRAttack) })
	case "AI_HELLBAT":
		return b.ranged(func() (d1Act, bool) { return b.missile(common.MisFireball, 0, common.D1ModeRAttack) })
	case "AI_HORKDEMON": // the unique Hork Demon fights like any skeleton until its own behaviour is ported
		return b.round(true)
	case "AI_BONED": // MAI_BoneDemon: MAI_RoundRanged with a special attack
		return b.roundRanged(common.MisFlare, 4, 1)
	case "AI_MAGMA":
		return b.roundRanged(common.MisMagmaball, 4, 0)
	case "AI_STORM":
		return b.roundRanged(common.MisLightCtrl2, 4, 0)
	case "AI_ACID":
		return b.roundRanged(common.MisAcid, 4, 1)
	case "AI_MEGA":
		return b.rr2(common.MisFlamec)
	case "AI_FAT":
		return b.fat()
	case "AI_GARG":
		return b.garg()
	case "AI_RHINO":
		return b.rhino()
	case "AI_SNAKE":
		return b.snake()
	case "AI_COUNSLR":
		return b.counselor()
	case "AI_CLEAVER":
		if mon.Squelch == 0 {
			return noAct, false
		}
		if b.near(2) {
			return b.attack()
		}
		return b.callWalk(b.dirTo(mon.Last))
	case "AI_SKELKING":
		return b.skelKing()
	case "AI_ZHAR", "AI_LAZURUS": // MAI_Zhar, MAI_Lazurus: talk, then MAI_Counselor
		if b.talker() {
			return b.counselor()
		}
		return noAct, false
	case "AI_LAZHELP": // MAI_Lazhelp fights like a succubus once Lazarus has spoken
		if b.talker() {
			return b.ranged(func() (d1Act, bool) { return b.missile(common.MisFlare, 0, common.D1ModeRSpAttack) })
		}
		return noAct, false
	case "AI_WARLORD":
		if b.talker() {
			return b.skelSd()
		}
		return noAct, false
	case "AI_GARBUD":
		if b.talker() {
			return b.round(true)
		}
		return noAct, false
	case "AI_SNOTSPIL":
		if b.talker() {
			return b.fallen()
		}
		return noAct, false
	case "AI_LACHDAN": // MAI_Lachdanan never fights
		b.talker()
		return noAct, false
	case "AI_DIABLO":
		return b.roundRanged(common.MisApoca, 40, 0)
	}
	return b.skelSd()
}

// talker runs a quest boss' talk states once per turn, so a boss that just spoke fights from its next turn on;
// true once it fights (Factory.D1TalkerTurn).
func (b *d1Brain) talker() bool {
	if b.fights == nil {
		fights := b.g.factory.D1TalkerTurn(b.a, b.g.state.GetPlayer().CanSee(b.pos))
		b.fights = &fights
	}
	return *b.fights
}

// skelKing ports MAI_SkelKing: circle now and then, raise skeletons from afar, hit up close.
// ponytail: D1's same-room check (dTransVal) counts as always true, like the other circling AIs.
func (b *d1Brain) skelKing() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	v := b.rnd(100)
	if b.near(2) || mon.Squelch != 255 {
		mon.Goal = 1
	} else if mon.Goal == 4 || !b.near(3) && b.rnd(4) == 0 {
		if mon.Goal != 4 {
			mon.GoalVar1, mon.GoalVar2 = 0, b.rnd(2)
		}
		mon.Goal = 4
		_, dirOK := b.step(md)
		if mon.GoalVar1++; mon.GoalVar1-1 >= 2*max(iabs(b.dx), iabs(b.dy)) && dirOK {
			mon.Goal = 1
		} else if act, ok := b.roundWalk(md); ok {
			return act, true
		} else {
			return b.delay(b.rnd(10) + 10)
		}
	}
	if mon.Goal != 1 {
		return noAct, false
	}
	switch {
	case (!b.near(3) && v < 4*mon.Int+35 || v < 6) && b.lineClear(b.enemy):
		if p, ok := b.step(md); ok {
			return d1Act{cost: b.attackCost(), mode: common.D1ModeSpStand, run: func() { b.g.factory.SpawnD1Skeleton(p) }}, true
		}
	case b.near(2):
		if v < mon.Int+20 {
			mon.Dir = md
			return b.attack()
		}
	default:
		if v = b.rnd(100); v < mon.Int+25 || b.justWalked() && v < mon.Int+75 {
			return b.callWalk(md)
		}
		return b.delay(b.rnd(10) + 10)
	}
	return noAct, false
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (b *d1Brain) rnd(n int) int {
	if n <= 0 {
		return 0
	}
	return b.g.randomSource.Intn(n)
}

func (b *d1Brain) near(r int) bool { return iabs(b.dx) < r && iabs(b.dy) < r }
func (b *d1Brain) stoodLong() bool { return b.mon.StandTicks > 20 }
func (b *d1Brain) justWalked() bool {
	return b.mon.LastMode == common.D1ModeWalk && b.mon.StandTicks == 0
}
func (b *d1Brain) walkCost() int   { return 100 / b.a.GetMoveSpeed() }
func (b *d1Brain) attackCost() int { return 100 / b.a.GetAttackSpeed(common.MeleeAttack) }
func (b *d1Brain) dirTo(p geometry.Point) int {
	return common.D1DirTo(b.pos, p)
}

func (b *d1Brain) delay(ticks int) (d1Act, bool) {
	return d1Act{cost: max(ticks*10/d1.RefWalkTicks, 1), mode: common.D1ModeDelay, run: func() {}}, true
}

func (b *d1Brain) attack() (d1Act, bool) {
	target := b.target
	if b.g.state.GetMap().MoveDistance(b.pos, b.enemy) > 1 {
		return noAct, false
	}
	return d1Act{cost: b.attackCost(), mode: common.D1ModeAttack, run: func() {
		b.g.actionBroker.Execute(common.NewMeleeAttackAction(common.NewActorCause("Melee attack", b.a), b.a, target))
	}}, true
}

// golemOnMap is the closest MFLAG_GOLEM monster to pos: the player's golem or a berserk monster.
func (g *GameInstance) golemOnMap(pos geometry.Point) *common.Actor {
	var best *common.Actor
	for _, a := range g.state.GetMap().Actors() {
		if mon := a.D1Monster(); mon != nil && (mon.Golem || mon.Berserk) && a.IsAlive() && a.MapPosition() != pos &&
			(best == nil || geometry.DistanceSquared(pos, a.MapPosition()) < geometry.DistanceSquared(pos, best.MapPosition())) {
			best = a
		}
	}
	return best
}

// golem ports MAI_Golum: fight the closest monster in line, otherwise stay with the player. A berserk monster
// (Hellfire) fights the same way but doesn't follow the player.
func (b *d1Brain) golem() (int, func()) {
	player := b.g.state.GetPlayer()
	for _, a := range b.g.state.GetMap().Actors() {
		mon := a.D1Monster()
		if mon == nil || mon.Golem || mon.Berserk || !a.IsAlive() || geometry.DistanceChebyshev(b.pos, a.MapPosition()) > 15 || !b.lineClear(a.MapPosition()) {
			continue
		}
		if b.target == nil || geometry.DistanceSquared(b.pos, a.MapPosition()) < geometry.DistanceSquared(b.pos, b.enemy) {
			b.enemy, b.target = a.MapPosition(), a
		}
	}
	act, ok := noAct, false
	if b.target != nil {
		if act, ok = b.attack(); !ok {
			act, ok = b.callWalk(b.dirTo(b.enemy))
		}
	} else if b.mon.Golem && geometry.DistanceChebyshev(b.pos, player.MapPosition()) > 2 {
		act, ok = b.callWalk(b.dirTo(player.MapPosition()))
	}
	if !ok {
		return 10, func() {}
	}
	return act.cost, act.run
}

func (b *d1Brain) shoot() (d1Act, bool) {
	target := b.enemy
	return d1Act{cost: 100 / b.a.GetAttackSpeed(common.RangedAttack), mode: common.D1ModeRAttack, run: func() {
		b.g.actionBroker.Execute(common.NewRangedAttackAction(common.NewActorCause("Ranged attack", b.a), b.a, target))
	}}, true
}

// step ports DirOK: the neighbour in direction d, and whether the monster may walk there.
func (b *d1Brain) step(d int) (geometry.Point, bool) {
	m := b.g.state.GetMap()
	p := b.pos.Add(common.D1Dirs[(d%8+8)%8])
	if m.MoveDistance(b.pos, p) != 1 {
		return p, false
	}
	if _, isTransition := m.GetTransitionAt(p); isTransition {
		return p, false
	}
	if b.g.state.IsClosedDoorAt(p) {
		return p, b.mon.Data.Flags&d1.MFlagCanOpenDoor != 0
	}
	return p, m.IsCurrentlyPassable(p)
}

func (b *d1Brain) walkTo(p geometry.Point) d1Act {
	return d1Act{cost: b.walkCost(), mode: common.D1ModeWalk, run: func() {
		b.mon.Dir = common.D1DirTo(b.pos, p)
		if b.g.state.IsClosedDoorAt(p) {
			b.g.actionBroker.Execute(common.NewOpenDoorAction(common.AbstractCause("OpenDoor"), b.a, p))
			return
		}
		b.g.actionBroker.Execute(common.NewMoveAction(common.AbstractCause("Movement"), b.a, p, false))
	}}
}

func (b *d1Brain) dumbWalk(d int) (d1Act, bool) {
	if p, ok := b.step(d); ok {
		return b.walkTo(p), true
	}
	return noAct, false
}

func (b *d1Brain) firstWalk(dirs ...int) (d1Act, bool) {
	for _, d := range dirs {
		if act, ok := b.dumbWalk(d); ok {
			return act, true
		}
	}
	return noAct, false
}

// callWalk ports M_CallWalk: the direction, then its neighbours, then the next ring, in random side order.
func (b *d1Brain) callWalk(d int) (d1Act, bool) {
	dirs := []int{d, d - 1, d + 1}
	if b.rnd(2) == 0 {
		dirs[1], dirs[2] = d+1, d-1
	}
	if b.rnd(2) != 0 {
		dirs = append(dirs, d+2, d-2)
	} else {
		dirs = append(dirs, d-2, d+2)
	}
	return b.firstWalk(dirs...)
}

// roundWalk ports M_RoundWalk: circle the enemy, flipping direction when blocked.
func (b *d1Brain) roundWalk(md int) (d1Act, bool) {
	mon := b.mon
	if mon.GoalVar2 != 0 {
		md -= 2
		if act, ok := b.firstWalk(md, md+1, md+2); ok {
			return act, true
		}
	} else {
		md += 2
		if act, ok := b.firstWalk(md, md-1, md-2); ok {
			return act, true
		}
	}
	mon.GoalVar2 = 1 - mon.GoalVar2
	return b.callWalk(md + 4)
}

// lineClear ports LineClear: nothing blocks the straight line to p. Lava and fences don't, like D1's missile table.
func (b *d1Brain) lineClear(p geometry.Point) bool {
	m := b.g.state.GetMap()
	line := geometry.LineOfSight(b.pos, p, func(q geometry.Point) bool {
		return q == b.pos || q == p || m.IsTransparent(q) && !m.IsActorAt(q) && (!m.IsObjectAt(q) || m.IsWalkable(q))
	})
	return len(line) > 0 && line[len(line)-1] == p
}

// path ports MAI_Path for monsters with the search flag: pathfind after 5 ticks without a clear line.
func (b *d1Brain) path() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 || mon.Goal != 1 && mon.Goal != 4 && mon.Goal != 5 {
		return noAct, false
	}
	if !b.lineClear(b.enemy) || mon.PathCount >= 5 && mon.PathCount < 8 {
		mon.PathCount++
		if mon.PathCount < 5 {
			return noAct, false
		}
		if next := b.g.GetMoveTowardsPlayer(b.pos); next != b.pos {
			if _, ok := b.step(b.dirTo(next)); ok {
				return b.walkTo(next), true
			}
		}
		return noAct, false
	}
	mon.PathCount = 0
	return noAct, false
}

func (b *d1Brain) zombie() (d1Act, bool) {
	mon := b.mon
	if !b.g.state.GetPlayer().CanSee(b.pos) {
		return noAct, false
	}
	v := b.rnd(100)
	if b.near(2) {
		if v < 2*mon.Int+10 {
			return b.attack()
		}
		return noAct, false
	}
	if v >= 2*mon.Int+10 {
		return noAct, false
	}
	if b.near(2*mon.Int + 4) {
		return b.callWalk(b.dirTo(b.enemy))
	}
	md := mon.Dir
	if b.rnd(100) < 2*mon.Int+20 {
		md = b.rnd(8)
	}
	return b.dumbWalk(md)
}

func (b *d1Brain) skelSd() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	mon.Dir = md
	if b.near(2) {
		if mon.LastMode == common.D1ModeDelay || b.rnd(100) < 2*mon.Int+20 {
			return b.attack()
		}
		return b.delay(b.rnd(10) + 10 - 2*mon.Int)
	}
	if mon.LastMode != common.D1ModeDelay && b.rnd(100) < 35-4*mon.Int {
		return b.delay(b.rnd(10) + 15 - 2*mon.Int)
	}
	return b.callWalk(md)
}

func (b *d1Brain) fallen() (d1Act, bool) {
	mon := b.mon
	if mon.Goal == 5 {
		if mon.GoalVar1 != 0 {
			mon.GoalVar1--
		} else {
			mon.Goal = 1
		}
	}
	if mon.Squelch == 0 {
		return noAct, false
	}
	if mon.Goal == 2 {
		if mon.GoalVar1 == 0 {
			mon.Goal = 1
			mon.Dir += 4
		} else {
			mon.GoalVar1--
		}
	}
	// D1 howls once at the end of each stand animation cycle with a 1 in 4 chance.
	if cycle := mon.Data.Frames[0] * max(mon.Data.Rate[0], 1); b.rnd(cycle) == 0 && b.rnd(4) == 0 {
		rad := 2*mon.Int + 4
		return d1Act{cost: b.attackCost(), mode: common.D1ModeSpStand, run: func() {
			if !mon.NoHeal { // MAI_Fallen: the howl itself heals the howler by 2*int+2, capped at max
				b.a.GetStats().GetResource(stats.Health).Increase(float64(2*mon.Int + 2))
			}
			for _, other := range b.g.state.GetMap().Actors() {
				om := other.D1Monster()
				if om != nil && om.AI == "AI_FALLEN" && geometry.DistanceChebyshev(other.MapPosition(), b.pos) <= rad {
					om.Goal, om.GoalVar1 = 5, 5*(6*mon.Int+21)
				}
			}
		}}, true
	}
	switch mon.Goal {
	case 2:
		return b.callWalk(mon.Dir)
	case 5:
		if b.near(2) {
			return b.attack()
		}
		return b.callWalk(b.dirTo(b.enemy))
	}
	return b.skelSd()
}

func (b *d1Brain) scav() (d1Act, bool) {
	mon := b.mon
	hp := b.a.GetStats().GetResource(stats.Health)
	if hp.Value() < hp.GetMax()/2 && mon.Goal != 3 {
		mon.Leader = nil
		mon.Goal, mon.GoalVar3 = 3, 10
	}
	if mon.Goal == 3 && mon.GoalVar3 != 0 {
		mon.GoalVar3--
		m := b.g.state.GetMap()
		if m.IsDownedActorAt(b.pos) {
			return d1Act{cost: b.attackCost(), mode: common.D1ModeSAttack, run: func() {
				hp.Increase(1)
				if hp.Value() >= hp.GetMax()/2+hp.GetMax()/4 {
					mon.Goal, mon.GoalVar1, mon.GoalVar2 = 1, 0, 0
				}
			}}, true
		}
		if mon.GoalVar1 == 0 {
			// ponytail: D1's corpse scan (with its offset bug) is simplified to the nearest corpse within 4.
			best := -1
			for y := -4; y <= 4; y++ {
				for x := -4; x <= 4; x++ {
					p := b.pos.Add(geometry.Point{X: x, Y: y})
					if d := max(iabs(x), iabs(y)); m.Contains(p) && m.IsDownedActorAt(p) && (best < 0 || d < best) && b.lineClear(p) {
						best, mon.GoalVar1, mon.GoalVar2 = d, p.X+1, p.Y+1
					}
				}
			}
		}
		if mon.GoalVar1 != 0 {
			target := geometry.Point{X: mon.GoalVar1 - 1, Y: mon.GoalVar2 - 1}
			mon.Dir = b.dirTo(target)
			if act, ok := b.callWalk(mon.Dir); ok {
				return act, true
			}
		}
	}
	return b.skelSd()
}

func (b *d1Brain) skelBow() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(b.enemy)
	mon.Dir = md
	v := b.rnd(100)
	if b.near(4) && (b.stoodLong() && v < 2*mon.Int+13 || b.justWalked() && v < 2*mon.Int+63) {
		if act, ok := b.dumbWalk(md + 4); ok {
			return act, true
		}
	}
	if b.rnd(100) < 2*mon.Int+3 && b.lineClear(b.enemy) {
		return b.shoot()
	}
	return noAct, false
}

func (b *d1Brain) bat() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	mon.Dir = md
	v := b.rnd(100)
	if mon.Goal == 2 {
		if mon.GoalVar1 == 0 {
			mon.GoalVar1++
			return b.callWalk(md + 4)
		}
		mon.Goal = 1
		if b.rnd(2) != 0 {
			return b.callWalk(md - 1)
		}
		return b.callWalk(md + 1)
	}
	if mon.Data.ID == "MT_GLOOM" && !b.near(5) && v < 4*mon.Int+33 && b.lineWalkable() {
		return b.charge(false, false)
	}
	if !b.near(2) {
		if b.stoodLong() && v < mon.Int+13 || b.justWalked() && v < mon.Int+63 {
			return b.callWalk(md)
		}
		return noAct, false
	}
	if v < 4*mon.Int+8 {
		if act, ok := b.attack(); ok {
			mon.Goal, mon.GoalVar1 = 2, 0
			if mon.Data.ID == "MT_FAMILIAR" && b.target == b.g.state.GetPlayer() {
				target, run := b.enemy, act.run
				act.run = func() {
					run()
					b.g.actionBroker.Execute(common.NewMonsterMissileAction(b.a, common.MisLightning, target, b.rnd(10)+1))
				}
			}
			return act, true
		}
	}
	return noAct, false
}

func (b *d1Brain) fade(out bool) (d1Act, bool) {
	mode := common.D1ModeFadeIn
	if out {
		mode = common.D1ModeFadeOut
	}
	return d1Act{cost: b.attackCost(), mode: mode, run: func() {
		if out {
			b.a.AddStatusEffect(common.StatusInvisible, -1)
		} else {
			b.a.RemoveStatusEffect(common.StatusInvisible)
		}
	}}, true
}

// sneak ports MAI_Sneak (Hidden family). ponytail: D1 also idles in total darkness; not ported.
func (b *d1Brain) sneak() (d1Act, bool) {
	mon := b.mon
	md := b.dirTo(b.enemy)
	dist := 5 - mon.Int
	if mon.LastMode == common.D1ModeGotHit {
		mon.Goal, mon.GoalVar1 = 2, 0
	} else if !b.near(dist+3) || mon.GoalVar1 > 8 {
		mon.Goal, mon.GoalVar1 = 1, 0
	}
	if mon.Goal == 2 {
		md += 4
		if mon.Data.ID == "MT_UNSEEN" {
			if b.rnd(2) != 0 {
				md--
			} else {
				md++
			}
		}
	}
	mon.Dir = md
	v := b.rnd(100)
	hidden := b.a.HasStatusEffect(common.StatusInvisible)
	switch {
	case b.near(dist) && hidden:
		return b.fade(false)
	case !b.near(dist+1) && !hidden:
		return b.fade(true)
	case mon.Goal == 2 || !b.near(2) && (b.stoodLong() && v < 4*mon.Int+14 || b.justWalked() && v < 4*mon.Int+64):
		mon.GoalVar1++
		if act, ok := b.callWalk(md); ok {
			return act, true
		}
	}
	if b.near(2) && v < 4*mon.Int+10 {
		return b.attack()
	}
	return noAct, false
}

// round ports MAI_Round (goat men, gargoyles): close in, sometimes circle around. ponytail: D1's same-room check
// (dTransVal) is treated as always true here and in the other circling AIs.
func (b *d1Brain) round(special bool) (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	v := b.rnd(100)
	if b.near(2) || mon.Squelch != 255 {
		mon.Goal = 1
	} else if mon.Goal == 4 || !b.near(4) && b.rnd(4) == 0 {
		if mon.Goal != 4 {
			mon.GoalVar1, mon.GoalVar2 = 0, b.rnd(2)
		}
		mon.Goal = 4
		dist := max(iabs(b.dx), iabs(b.dy))
		_, dirOK := b.step(md)
		mon.GoalVar1++
		if mon.GoalVar1-1 >= 2*dist && dirOK {
			mon.Goal = 1
		} else if act, ok := b.roundWalk(md); ok {
			return act, true
		} else {
			return b.delay(b.rnd(10) + 10)
		}
	}
	if mon.Goal == 1 {
		if b.near(2) {
			if v < 2*mon.Int+23 {
				mon.Dir = md
				if hp := b.a.GetStats().GetResource(stats.Health); special && hp.Value() < hp.GetMax()/2 && b.rnd(2) != 0 {
					return b.specialAttack()
				}
				return b.attack()
			}
		} else if b.stoodLong() && v < 2*mon.Int+28 || b.justWalked() && v < 2*mon.Int+78 {
			return b.callWalk(md)
		}
	}
	return noAct, false
}

// ranged ports MAI_Ranged (goat archers, succubi, unique acid beasts): keep distance, fire on a clear line.
func (b *d1Brain) ranged(fire func() (d1Act, bool)) (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 255 {
		md := b.dirTo(b.enemy)
		mon.Dir = md
		if mon.LastMode == common.D1ModeRAttack {
			return b.delay(b.rnd(20))
		}
		if b.near(4) && b.rnd(100) < 10*(mon.Int+7) {
			if act, ok := b.callWalk(md + 4); ok {
				return act, true
			}
		}
		if b.lineClear(b.enemy) {
			return fire()
		}
		return noAct, false
	}
	if mon.Squelch != 0 {
		return b.callWalk(b.dirTo(mon.Last))
	}
	return noAct, false
}
