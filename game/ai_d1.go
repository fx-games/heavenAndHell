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
	b.chooseEnemy(player)
	b.dx, b.dy = b.pos.X-b.enemy.X, b.pos.Y-b.enemy.Y
	switch {
	case b.target != player: // ProcessMonsters: a monster fighting the golem always knows where it is
		mon.Last = b.enemy
	case player.CanSee(b.pos):
		if mon.Squelch == 0 {
			g.factory.D1MonsterGreeting(a)
		}
		mon.Squelch, mon.Last = 255, b.enemy
	default:
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
	return b.commit(act)
}

// chooseEnemy ports M_Enemy for a monster that isn't a golem: the player, or the golem when it stands right next to
// the monster (any golem, for the ranged AIs). The one in the monster's own room wins, then the closer one.
func (b *d1Brain) chooseEnemy(player *common.Actor) {
	best, bestSame := -1, false
	consider := func(a *common.Actor) {
		p := a.MapPosition()
		same := common.D1SameRoom(&b.g.state, b.pos, p)
		dist := max(iabs(b.pos.X-p.X), iabs(b.pos.Y-p.Y))
		if same && !bestSame || (same || !bestSame) && dist < best || best == -1 {
			b.enemy, b.target, best, bestSame = p, a, dist, same
		}
	}
	consider(player)
	ranged := map[string]bool{"AI_SKELBOW": true, "AI_GOATBOW": true, "AI_SUCC": true, "AI_LAZHELP": true}[b.mon.AI] // M_Ranged
	for _, a := range b.g.state.GetMap().Actors() {
		if mon := a.D1Monster(); a != b.a && mon != nil && mon.Golem && a.IsAlive() &&
			(ranged || geometry.DistanceChebyshev(b.pos, a.MapPosition()) < 2) {
			consider(a)
		}
	}
}

// checkDoors ports MonstCheckDoors: a monster that has lost sight of the player opens the closed doors it stands
// beside - Cathedral and Catacombs doors facing it along x (left) or y (right), the Caves' the other way round.
func (b *d1Brain) checkDoors() {
	m := b.g.state.GetMap()
	for _, d := range common.D1Dirs {
		p := b.pos.Add(d)
		o, _ := m.TryGetObjectAt(p)
		door, ok := o.(*common.Door)
		if !ok || door.D1 == nil || !door.IsClosed() {
			continue
		}
		dpx, dpy := iabs(p.X-b.pos.X), iabs(p.Y-b.pos.Y)
		alongX := dpx == 1 && dpy <= 1
		alongY := dpx <= 1 && dpy == 1
		switch door.D1.Kind {
		case d1.ObjL1LDOOR, d1.ObjL2LDOOR, d1.ObjL3RDOOR:
			ok = alongX
		case d1.ObjL1RDOOR, d1.ObjL2RDOOR, d1.ObjL3LDOOR:
			ok = alongY
		default:
			ok = false
		}
		if ok {
			b.g.actionBroker.Execute(common.NewOpenDoorAction(common.AbstractCause("OpenDoor"), b.a, p))
		}
	}
}

func (b *d1Brain) commit(act d1Act) (int, func()) {
	return act.cost, func() {
		b.modeSound(act.mode)
		act.run()
		b.mon.LastMode, b.mon.StandTicks = act.mode, 0
		common.D1GroupUnity(b.g.state.GetMap(), b.a)
	}
}

// modeSound is the PlayEffect a mode starts with that isn't a melee swing's (MeleeAttackAction plays those):
// M_DoRAttack's attack sound, M_DoRSpAttack's and M_DoSpStand's special one, and a charge's (MAI_Rhino).
func (b *d1Brain) modeSound(mode int) {
	f := b.g.factory
	switch mode {
	case common.D1ModeRAttack:
		f.PlayD1MonsterSound(b.a, "attack")
	case common.D1ModeRSpAttack, common.D1ModeSpStand:
		f.PlayD1MonsterSound(b.a, "special")
	case common.D1ModeCharge:
		if !f.PlayD1MonsterSound(b.a, "special") {
			f.PlayD1MonsterSound(b.a, "attack")
		}
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
		return b.roundRanged(common.MisFlare, true, 4, 1)
	case "AI_MAGMA":
		return b.roundRanged(common.MisMagmaball, true, 4, 0)
	case "AI_STORM":
		return b.roundRanged(common.MisLightCtrl2, true, 4, 0)
	case "AI_ACID":
		return b.roundRanged(common.MisAcid, false, 4, 1)
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
		return b.roundRanged(common.MisApoca, false, 40, 0)
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
func (b *d1Brain) skelKing() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	if mon.Squelch < 255 {
		b.checkDoors()
	}
	v := b.rnd(100)
	if b.near(2) || mon.Squelch != 255 || !b.sameRoom() {
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
func (b *d1Brain) sameRoom() bool  { return common.D1SameRoom(&b.g.state, b.pos, b.enemy) }
func (b *d1Brain) stoodLong() bool { return b.mon.StandTicks > 20 }
func (b *d1Brain) justWalked() bool {
	return b.mon.LastMode == common.D1ModeWalk && b.mon.StandTicks == 0
}
func (b *d1Brain) walkCost() int   { return b.a.MoveCost() }
func (b *d1Brain) attackCost() int { return b.a.AttackCost(common.MeleeAttack) }
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

// d1Talkers are M_Talker's AIs: quest monsters that speak before they fight.
var d1Talkers = map[string]bool{"AI_LAZURUS": true, "AI_WARLORD": true, "AI_GARBUD": true, "AI_ZHAR": true,
	"AI_SNOTSPIL": true, "AI_LACHDAN": true, "AI_LAZHELP": true}

// golem ports MAI_Golum: M_Enemy over every other monster on the level (one with something left to say excepted),
// its own room first, then the closest; strike it when adjacent, waking it and the monsters around the golem;
// otherwise path to it (MAI_Path), and failing that walk the way the player faces (east or west). A berserk monster (Hellfire)
// fights the same way but has no player to follow.
func (b *d1Brain) golem() (int, func()) {
	player := b.g.state.GetPlayer()
	best, bestSame := -1, false
	for _, a := range b.g.state.GetMap().Actors() {
		mon := a.D1Monster()
		if a == b.a || mon == nil || mon.Golem || !a.IsAlive() || d1Talkers[mon.AI] && mon.TalkMsg != "" {
			continue
		}
		p := a.MapPosition()
		same := common.D1SameRoom(&b.g.state, b.pos, p)
		dist := max(iabs(b.pos.X-p.X), iabs(b.pos.Y-p.Y))
		if same && !bestSame || (same || !bestSame) && dist < best || best == -1 {
			b.enemy, b.target, best, bestSame = p, a, dist, same
		}
	}
	act, ok := noAct, false
	switch {
	case b.target != nil && best < 2:
		if tm := b.target.D1Monster(); tm.Squelch == 0 {
			tm.Squelch, tm.Last = 255, b.pos
			// D1 then wakes the 5x5 block around the golem, but indexes monster[] off by one (monster[dMonster]
			// rather than monster[dMonster-1]), so it wakes whichever monster comes next in the table: the one
			// with the next SpawnIndex. Monsters spawned after the level was made have no slot to follow.
			next := map[int]bool{}
			for _, a := range b.g.state.GetMap().Actors() {
				if om := a.D1Monster(); om != nil && !om.Golem && om.SpawnIndex >= 0 && geometry.DistanceChebyshev(a.MapPosition(), b.pos) <= 2 {
					next[om.SpawnIndex+1] = true
				}
			}
			for _, a := range b.g.state.GetMap().Actors() {
				if om := a.D1Monster(); om != nil && !om.Golem && om.SpawnIndex >= 0 && next[om.SpawnIndex] {
					om.Squelch = 255
				}
			}
		}
		act, ok = b.attack()
	case b.target != nil:
		act, ok = b.golemPath()
	}
	if !ok && b.mon.Golem {
		if b.mon.PathCount++; b.mon.PathCount > 8 {
			b.mon.PathCount = 5
		}
		facing := 7 // D1's _pdir; this engine faces east or west only
		if player.FlipX() {
			facing = 3
		}
		if act, ok = b.callWalk(facing); !ok {
			md := b.dirTo(b.enemy)
			for j := 0; j < 8 && !ok; j++ {
				act, ok = b.dumbWalk(md + j)
			}
		}
	}
	if !ok {
		return 10, func() {}
	}
	return act.cost, act.run
}

// golemPath is MAI_Path for the golem: after five turns without a walkable line to its enemy it takes the first
// step of a path of up to 25 (FindPath's MAX_PATH_LENGTH). Unlike a monster's, its count never resets.
func (b *d1Brain) golemPath() (d1Act, bool) {
	mon := b.mon
	if b.lineFree() && (mon.PathCount < 5 || mon.PathCount >= 8) {
		return noAct, false
	}
	if mon.Data.Flags&d1.MFlagCanOpenDoor != 0 {
		b.checkDoors()
	}
	if mon.PathCount++; mon.PathCount < 5 {
		return noAct, false
	}
	return b.pathStep()
}

func (b *d1Brain) shoot() (d1Act, bool) {
	target := b.enemy
	return d1Act{cost: b.a.AttackCost(common.RangedAttack), mode: common.D1ModeRAttack, run: func() {
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
		return p, b.mon.Data.Flags&d1.MFlagCanOpenDoor != 0 && common.D1PackStepOK(m, b.a, p)
	}
	return p, m.IsCurrentlyPassable(p) && !common.D1FireWallBlocks(m, p, b.mon) && common.D1PackStepOK(m, b.a, p)
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

// lineNoSolid ports LineClearF(CheckNoSolid): no solid piece of the level between the monster and p.
func (b *d1Brain) lineNoSolid(p geometry.Point) bool {
	m := b.g.state.GetMap()
	line := geometry.LineOfSight(b.pos, p, func(q geometry.Point) bool { return q == b.pos || q == p || m.IsWalkable(q) || m.IsTransparent(q) })
	return len(line) > 0 && line[len(line)-1] == p
}

// lineFree ports LineClearF1(PosOkMonst2): no wall, solid object or fire wall between the monster and its enemy.
// Unlike lineWalkable, other creatures don't block it.
func (b *d1Brain) lineFree() bool {
	m := b.g.state.GetMap()
	line := geometry.LineOfSight(b.pos, b.enemy, func(q geometry.Point) bool {
		return q == b.pos || q == b.enemy || m.IsWalkable(q) && !common.D1FireWallBlocks(m, q, b.mon)
	})
	return len(line) > 0 && line[len(line)-1] == b.enemy
}

// lineClear ports LineClear: nothing blocks the straight line to p. Lava and fences don't, like D1's missile table.
func (b *d1Brain) lineClear(p geometry.Point) bool {
	m := b.g.state.GetMap()
	line := geometry.LineOfSight(b.pos, p, func(q geometry.Point) bool {
		return q == b.pos || q == p || m.IsTransparent(q) && !m.IsActorAt(q) && (!m.IsObjectAt(q) || m.IsWalkable(q))
	})
	return len(line) > 0 && line[len(line)-1] == p
}

// path ports MAI_Path for monsters with the search flag: pathfind after 5 ticks without a walkable line.
func (b *d1Brain) path() (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 || mon.Goal != 1 && mon.Goal != 4 && mon.Goal != 5 {
		return noAct, false
	}
	if !b.lineFree() || mon.PathCount >= 5 && mon.PathCount < 8 {
		if mon.Data.Flags&d1.MFlagCanOpenDoor != 0 {
			b.checkDoors()
		}
		mon.PathCount++
		if mon.PathCount < 5 {
			return noAct, false
		}
		return b.pathStep()
	}
	mon.PathCount = 0
	return noAct, false
}

// pathStep ports M_PathWalk: the first step of a path to the enemy of at most 25 (FindPath's MAX_PATH_LENGTH). The
// player's own distance map serves when the enemy is the player.
func (b *d1Brain) pathStep() (d1Act, bool) {
	if b.target == b.g.state.GetPlayer() {
		if next := b.g.GetMoveTowardsPlayer(b.pos); next != b.pos {
			if _, ok := b.step(b.dirTo(next)); ok {
				return b.walkTo(next), true
			}
		}
		return noAct, false
	}
	dm := b.g.state.GetMap().GetDijkstraMapWithActorsNotBlocking(b.enemy, 25)
	here, ok := dm[b.pos]
	if !ok {
		return noAct, false
	}
	for d := 0; d < 8; d++ {
		if p, free := b.step(d); free {
			if n, on := dm[p]; on && n < here {
				return b.walkTo(p), true
			}
		}
	}
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
	if b.near(2*mon.Int + 4) { // MAI_Zombie only homes in from close by
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
			// D1 bounds-checks the offsets instead of the cells, so the howl only rallies the fallen ones 0..rad
			// cells to the south-east, the howler included
			for _, other := range b.g.state.GetMap().Actors() {
				om, d := other.D1Monster(), other.MapPosition().Sub(b.pos)
				if om != nil && om.AI == "AI_FALLEN" && d.X >= 0 && d.Y >= 0 && d.X <= rad && d.Y <= rad {
					om.Goal, om.GoalVar1 = 5, 30*mon.Int+105
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
		common.D1LeavePack(b.g.state.GetMap(), mon)
		mon.Goal, mon.GoalVar3 = 3, 10
	}
	if mon.Goal == 3 && mon.GoalVar3 != 0 {
		mon.GoalVar3--
		m := b.g.state.GetMap()
		if m.IsDownedActorAt(b.pos) {
			return d1Act{cost: b.attackCost(), mode: common.D1ModeSAttack, run: func() {
				if !mon.NoHeal {
					hp.Increase(1)
				}
				if hp.Value() >= hp.GetMax()/2+hp.GetMax()/4 {
					mon.Goal, mon.GoalVar1, mon.GoalVar2 = 1, 0, 0
				}
			}}, true
		}
		if mon.GoalVar1 == 0 {
			// D1 checks the offsets against the dungeon's bounds instead of the cells, so it only ever looks at
			// the corpses 0-4 cells to the south-east; a coin decides whether it scans from near or from far.
			order := []int{0, 1, 2, 3, 4}
			if b.rnd(2) == 0 {
				order = []int{4, 3, 2, 1, 0}
			}
		scan:
			for _, y := range order {
				for _, x := range order {
					p := b.pos.Add(geometry.Point{X: x, Y: y})
					if m.Contains(p) && m.IsDownedActorAt(p) && b.lineNoSolid(p) {
						mon.GoalVar1, mon.GoalVar2 = p.X+1, p.Y+1
						break scan
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
	// MAI_Bat doesn't return after the retreat step: a charge, walk or attack later in the same tick replaces it
	retreat, retreating := noAct, false
	if mon.Goal == 2 {
		if mon.GoalVar1 == 0 {
			retreat, retreating = b.callWalk(md + 4)
			mon.GoalVar1++
		} else {
			if b.rnd(2) != 0 {
				retreat, retreating = b.callWalk(md - 1)
			} else {
				retreat, retreating = b.callWalk(md + 1)
			}
			mon.Goal = 1
		}
	}
	if mon.Data.ID == "MT_GLOOM" && !b.near(5) && v < 4*mon.Int+33 && b.lineWalkable() {
		return b.charge(false, false)
	}
	if !b.near(2) {
		if b.stoodLong() && v < mon.Int+13 || b.justWalked() && v < mon.Int+63 {
			if act, ok := b.callWalk(md); ok {
				return act, true
			}
		}
		return retreat, retreating
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
	return retreat, retreating
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

// sneak ports MAI_Sneak (Hidden family). It does nothing in total darkness (dLight == lightmax): outside the
// hero's light radius and any lit room.
func (b *d1Brain) sneak() (d1Act, bool) {
	mon := b.mon
	player := b.g.state.GetPlayer()
	if player.Hero() != nil && geometry.Distance(b.pos, player.MapPosition()) > float64(player.HeroSheet().LightRad) && !b.g.state.GetMap().IsTileLit(b.pos) {
		return noAct, false
	}
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

// round ports MAI_Round (goat men, gargoyles): close in, sometimes circle around - only in the enemy's room
// (dTransVal), which also ends the circling once they no longer share one.
func (b *d1Brain) round(special bool) (d1Act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	if mon.Squelch < 255 {
		b.checkDoors()
	}
	v := b.rnd(100)
	if b.near(2) || mon.Squelch != 255 || !b.sameRoom() {
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
	if mon.Squelch == 255 || b.target != b.g.state.GetPlayer() {
		if mon.Squelch < 255 {
			b.checkDoors()
		}
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
