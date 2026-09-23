package game

import (
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/stats"
)

// The AIs of the monsters below the cathedral (monster.cpp MAI_*), and the attacks they need.

// missile fires a monster missile at the enemy (M_StartRAttack / M_StartRSpAttack). D1's monster missiles only
// hit players, so a monster busy with the golem doesn't cast.
func (b *brain) missile(mis common.MonsterMissile, dam, mode int) (act, bool) {
	if b.target != b.g.state.GetPlayer() {
		return noAct, false
	}
	target := b.enemy
	return act{cost: b.attackCost(), mode: mode, run: func() {
		b.mon.Dir = b.dirTo(target)
		b.g.actionBroker.Execute(common.NewMonsterMissileAction(b.a, mis, target, dam))
	}}, true
}

// specialAttack ports M_StartSpAttack: a melee attack with the second to-hit and damage values.
func (b *brain) specialAttack() (act, bool) {
	act, ok := b.attack()
	if ok {
		mon, run := b.mon, act.run
		act.mode = common.ModeSAttack
		act.run = func() {
			mon.HitWith = &[3]int{mon.Hit2, mon.MinDam2, mon.MaxDam2}
			run()
		}
	}
	return act, ok
}

// lineWalkable ports LineClearF1(PosOkMonst): every cell up to the enemy is free to walk on.
func (b *brain) lineWalkable() bool {
	m := b.g.state.GetMap()
	line := geometry.LineOfSight(b.pos, b.enemy, func(q geometry.Point) bool { return q == b.pos || q == b.enemy || m.IsCurrentlyPassable(q) })
	return len(line) > 0 && line[len(line)-1] == b.enemy
}

// charge ports MIS_RHINO and MissToMonst: rush along the line until something blocks, then maybe hit with the
// special damage (to-hit 500) and knock the player back a cell.
func (b *brain) charge(hit, knockback bool) (act, bool) {
	target, enemy := b.target, b.enemy
	return act{cost: b.walkCost(), mode: common.ModeCharge, run: func() {
		m := b.g.state.GetMap()
		last := b.pos
		for _, p := range geometry.LineOfSight(b.pos, enemy, func(geometry.Point) bool { return true })[1:] {
			if !m.IsCurrentlyPassable(p) {
				break
			}
			last = p
		}
		mon := b.mon
		mon.Dir = common.DirTo(b.pos, enemy)
		if last != b.pos {
			b.g.actionBroker.Execute(common.NewMoveAction(common.AbstractCause("Charge"), b.a, last, false))
		}
		if !hit || geometry.DistanceChebyshev(last, target.MapPosition()) > 1 {
			return
		}
		mon.HitWith = &[3]int{500, mon.MinDam2, mon.MaxDam2}
		b.g.actionBroker.Execute(common.NewMeleeAttackAction(common.NewActorCause("Charge", b.a), b.a, target))
		push := target.MapPosition().Add(geometry.Dirs[mon.Dir])
		if knockback && target.IsAlive() && m.IsCurrentlyPassable(push) {
			if _, isTransition := m.GetTransitionAt(push); !isTransition {
				b.g.actionBroker.Execute(common.NewMoveAction(common.AbstractCause("Knockback"), target, push, target == b.g.state.GetPlayer()))
			}
		}
		if knockback {
			common.StartPlrHit(b.g.state.Factory(), target, 0, 0, true) // the charge's forced flinch
		}
	}}, true
}

// circle is the "goal 4" block MAI_Round, MAI_RoundRanged, MAI_Rhino and MAI_RR2 share: once started, walk around
// the enemy until twice the distance is covered. It reports whether the monster is still circling.
func (b *brain) circle(md int, needDirOK bool) bool {
	mon := b.mon
	if mon.Goal != 4 {
		mon.GoalVar1, mon.GoalVar2 = 0, b.rnd(2)
	}
	mon.Goal = 4
	dist := max(iabs(b.dx), iabs(b.dy))
	_, dirOK := b.step(md)
	mon.GoalVar1++
	if mon.GoalVar1-1 >= 2*dist && (dirOK || !needDirOK) {
		mon.Goal = 1
		return false
	}
	return true
}

// roundRanged ports MAI_RoundRanged (magma, storm and acid demons).
func (b *brain) roundRanged(mis common.MonsterMissile, checkDoors bool, dam, less int) (act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	if checkDoors && mon.Squelch < 255 {
		b.checkDoors()
	}
	v := b.rnd(10000)
	cast := func() (act, bool) {
		if b.lineClear(b.enemy) {
			return b.missile(mis, dam, common.ModeRSpAttack)
		}
		return noAct, false
	}
	if b.near(2) || mon.Squelch != 255 || !b.sameRoom() {
		mon.Goal = 1
	} else if mon.Goal == 4 || !b.near(3) && b.rnd(4<<less) == 0 {
		if b.circle(md, true) {
			if v < 500*(mon.AILevel+1)>>less {
				if act, ok := cast(); ok {
					return act, true
				}
			}
			if act, ok := b.roundWalk(md); ok {
				return act, true
			}
		}
	}
	if mon.Goal == 1 {
		if !b.near(3) && v < 500*(mon.AILevel+2)>>less || v < 500*(mon.AILevel+1)>>less {
			if act, ok := cast(); ok {
				return act, true
			}
		}
		if b.near(2) {
			if v < 1000*(mon.AILevel+6) {
				mon.Dir = md
				return b.attack()
			}
		} else if act, ok := b.callWalk(md); ok { // D1 compares a 0-99 roll with 1000*(int+5): always walks
			return act, true
		}
	}
	return b.delay(b.rnd(10) + 5)
}

// rr2 ports MAI_RR2 (balrogs and their kin): fight like a skeleton from afar, flame and circle up close.
func (b *brain) rr2(mis common.MonsterMissile) (act, bool) {
	mon := b.mon
	if !b.near(5) { // MAI_RR2 doesn't return after MAI_SkelSd: when that finds nothing to do, it goes on
		if act, ok := b.skelSd(); ok {
			return act, true
		}
	}
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
	} else if mon.Goal == 4 || !b.near(3) {
		mon.GoalVar3 = 4
		if b.circle(md, true) && v < 5*(mon.AILevel+16) {
			if act, ok := b.roundWalk(md); ok {
				return act, true
			}
		}
	}
	if mon.Goal == 1 {
		flamed := mon.GoalVar3 == 4
		mon.GoalVar3 = 1
		if (!b.near(3) && v < 5*(mon.AILevel+2) || v < 5*(mon.AILevel+1) || flamed) && b.lineClear(b.enemy) {
			if act, ok := b.missile(mis, 0, common.ModeRSpAttack); ok {
				return act, true
			}
		}
		if b.near(2) {
			if b.rnd(100) < 10*(mon.AILevel+4) {
				mon.Dir = md
				if b.rnd(2) == 0 {
					if act, ok := b.missile(mis, 0, common.ModeRSpAttack); ok {
						return act, true
					}
				}
				return b.attack()
			}
		} else if v = b.rnd(100); v < 2*(5*mon.AILevel+25) || b.justWalked() && v < 2*(5*mon.AILevel+40) {
			return b.callWalk(md)
		}
	}
	return b.delay(b.rnd(10) + 5)
}

// fat ports MAI_Fat (overlords, mud men, toad demons, flayed ones).
func (b *brain) fat() (act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(b.enemy)
	mon.Dir = md
	v := b.rnd(100)
	switch {
	case !b.near(2):
		if b.stoodLong() && v < 4*mon.AILevel+20 || b.justWalked() && v < 4*mon.AILevel+70 {
			return b.callWalk(md)
		}
	case v < 4*mon.AILevel+15:
		return b.attack()
	case v < 4*mon.AILevel+20:
		return b.specialAttack()
	}
	return noAct, false
}

// garg ports MAI_Garg: gargoyles wait in stone until the enemy comes close, and go back to stone to heal.
func (b *brain) garg() (act, bool) {
	mon := b.mon
	hp := b.a.GetStats().GetResource(stats.Health)
	if mon.LastMode == common.ModeHeal && hp.Value() < hp.GetMax() { // M_DoHeal: a heal step per animation cycle
		return act{cost: 10, mode: common.ModeHeal, run: func() {
			hp.Increase(float64(max(hp.GetMax()/(16*(b.rnd(5)+4)), 1)))
		}}, true
	}
	if mon.Dormant {
		if mon.Squelch != 0 && b.near(mon.AILevel+2) {
			mon.Dormant = false
		}
		return noAct, false
	}
	if mon.Squelch == 0 {
		return noAct, false
	}
	if hp.Value() < hp.GetMax()/2 && !mon.NoHeal {
		mon.Goal = 2
	}
	if mon.Goal == 2 {
		last := b.pos.Sub(mon.Last)
		if iabs(last.X) < mon.AILevel+2 && iabs(last.Y) < mon.AILevel+2 {
			if act, ok := b.callWalk(b.dirTo(b.enemy) + 4); ok {
				return act, true
			}
			mon.Goal = 1
		} else {
			mon.Goal = 1
			return act{cost: 10, mode: common.ModeHeal, run: func() {}}, true
		}
	}
	return b.round(false)
}

// rhino ports MAI_Rhino (horned demons): circle, then charge from afar.
func (b *brain) rhino() (act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	if mon.Squelch < 255 {
		b.checkDoors()
	}
	v := b.rnd(100)
	if b.near(2) {
		mon.Goal = 1
	} else if mon.Goal == 4 || !b.near(5) && b.rnd(4) != 0 {
		if b.circle(md, false) && b.sameRoom() { // MAI_Rhino stops circling once they no longer share a room
			if act, ok := b.roundWalk(md); ok {
				return act, true
			}
			return b.delay(b.rnd(10) + 10)
		}
		mon.Goal = 1
	}
	if mon.Goal != 1 {
		return noAct, false
	}
	switch {
	case !b.near(5) && v < 2*mon.AILevel+43 && b.lineWalkable():
		return b.charge(true, true)
	case b.near(2):
		if v < 2*mon.AILevel+28 {
			mon.Dir = md
			return b.attack()
		}
		return noAct, false
	}
	if v = b.rnd(100); v < 2*mon.AILevel+33 || b.justWalked() && v < 2*mon.AILevel+83 {
		return b.callWalk(md)
	}
	return b.delay(b.rnd(10) + 10)
}

// snake ports MAI_Snake (cave vipers, drakes): slither in a zigzag, lunge from two cells away.
func (b *brain) snake() (act, bool) {
	pattern := [6]int{1, 1, 0, -1, -1, 0}
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	mon.Dir = md
	switch {
	case b.near(2):
		if mon.LastMode == common.ModeDelay || mon.LastMode == common.ModeCharge || b.rnd(100) < mon.AILevel+20 {
			return b.attack()
		}
		return b.delay(b.rnd(10) + 10 - mon.AILevel)
	case b.near(3) && mon.LastMode != common.ModeCharge && b.lineWalkable():
		return b.charge(true, false)
	case mon.LastMode != common.ModeDelay && b.rnd(100) < 35-2*mon.AILevel:
		return b.delay(b.rnd(10) + 15 - mon.AILevel)
	}
	md = (md + pattern[mon.GoalVar1] + 8) % 8
	mon.GoalVar1 = (mon.GoalVar1 + 1) % 6
	if d := (md - mon.GoalVar2 + 8) % 8; d > 0 {
		switch {
		case d < 4:
			mon.GoalVar2 = (mon.GoalVar2 + 1) % 8
		case d == 4:
			mon.GoalVar2 = md
		default:
			mon.GoalVar2 = (mon.GoalVar2 + 7) % 8
		}
	}
	if act, ok := b.dumbWalk(mon.GoalVar2); ok {
		return act, true
	}
	if b.rnd(2) != 0 { // M_CallWalk2
		return b.firstWalk(mon.Dir, mon.Dir-1, mon.Dir+1)
	}
	return b.firstWalk(mon.Dir, mon.Dir+1, mon.Dir-1)
}

var counselorMissiles = [4]common.MonsterMissile{common.MisFirebolt, common.MisCbolt, common.MisLightCtrl, common.MisFireball}

// counselor ports MAI_Counselor (counselors, magistrates, cabalists, advocates): cast from afar, flash up close,
// fade away to circle or flee.
func (b *brain) counselor() (act, bool) {
	mon := b.mon
	if mon.Squelch == 0 {
		return noAct, false
	}
	md := b.dirTo(mon.Last)
	if mon.Squelch < 255 {
		b.checkDoors()
	}
	v := b.rnd(100)
	switch mon.Goal {
	case 2:
		if mon.GoalVar1++; mon.GoalVar1-1 > 3 {
			mon.Goal = 1
			return b.fade(false)
		}
		if act, ok := b.callWalk(md + 4); ok {
			return act, true
		}
	case 4:
		dist := max(iabs(b.dx), iabs(b.dy))
		_, dirOK := b.step(md)
		if b.near(2) || mon.Squelch != 255 || !b.sameRoom() {
			mon.Goal = 1
			return b.fade(false)
		}
		if mon.GoalVar1++; mon.GoalVar1-1 >= 2*dist && dirOK {
			mon.Goal = 1
			return b.fade(false)
		}
		if act, ok := b.roundWalk(md); ok {
			return act, true
		}
	case 1:
		if b.near(2) {
			mon.Dir = md
			if hp := b.a.GetStats().GetResource(stats.Health); hp.Value() < hp.GetMax()/2 {
				mon.Goal, mon.GoalVar1 = 2, 0
				return b.fade(true)
			}
			if mon.LastMode == common.ModeDelay || b.rnd(100) < 2*mon.AILevel+20 {
				if act, ok := b.missile(common.MisFlash, 0, common.ModeRAttack); ok {
					return act, true
				}
				return b.attack()
			}
			return b.delay(b.rnd(10) + 2*(5-mon.AILevel))
		}
		if v < 5*(mon.AILevel+10) && b.lineClear(b.enemy) {
			dam := mon.MinDam + b.rnd(mon.MaxDam-mon.MinDam+1)
			if act, ok := b.missile(counselorMissiles[min(mon.AILevel, 3)], dam, common.ModeRAttack); ok {
				return act, true
			}
		}
		if b.rnd(100) < 30 {
			mon.Goal, mon.GoalVar1 = 4, 0
			return b.fade(true)
		}
		return b.delay(b.rnd(10) + 2*(5-mon.AILevel))
	}
	return b.delay(b.rnd(10) + 5)
}
