package common

import (
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

// A pack is a unique with mUnqAttr & 2 and the minions PlaceGroup tied to it. MonsterState keeps the ties:
// LeaderIdx is the leader's SpawnIndex (its slot on the level), LeaderFlag 1 while a minion keeps up with it and 2
// once it strayed, PackSize on the leader the minions at flag 1.

type packMap = gridmap.GridMap[*Actor, *Item, Object]

// packLeader is a minion's leader, if it still has one.
func packLeader(m *packMap, mon *Monster) *Actor {
	if mon.LeaderFlag == 0 {
		return nil
	}
	for _, a := range m.Actors() {
		if a.monster != nil && a.monster.SpawnIndex == mon.LeaderIdx && a.IsAlive() {
			return a
		}
	}
	return nil
}

// packMinions are a leader's minions at flag 1.
func packMinions(m *packMap, leader *Actor) (out []*Actor) {
	for _, a := range m.Actors() {
		if mon := a.monster; mon != nil && mon.LeaderFlag == 1 && mon.LeaderIdx == leader.monster.SpawnIndex && a != leader && a.IsAlive() {
			out = append(out, a)
		}
	}
	return
}

func packRuled(mon *Monster) bool { return mon.Unique != nil && mon.Unique.UnqAttr&2 != 0 }

// PackStepOK is DirOK's pack half for a step onto p: a minion keeps within 3 cells of its leader, and a leader
// with mUnqAttr & 2 only moves when its whole pack stands within 3 cells of where it goes.
func PackStepOK(m *packMap, a *Actor, p geometry.Point) bool {
	mon := a.monster
	if mon.LeaderFlag == 1 {
		if l := packLeader(m, mon); l != nil {
			d := p.Sub(l.MapPosition())
			return geometry.Abs(d.X) < 4 && geometry.Abs(d.Y) < 4
		}
		return true
	}
	if !packRuled(mon) {
		return true
	}
	n := 0
	for _, mi := range packMinions(m, a) {
		if d := mi.MapPosition().Sub(p); geometry.Abs(d.X) <= 3 && geometry.Abs(d.Y) <= 3 {
			n++
		}
	}
	return n == mon.PackSize
}

// GroupUnity ports GroupUnity, run after each thing a monster does: a minion out of its leader's sight strays
// (flag 2), one back in sight and within 3 cells rejoins; the more alert of leader and minion tells the other
// where the enemy was, and wakes a dormant gargoyle.
func GroupUnity(m *packMap, a *Actor) {
	mon := a.monster
	tell := func(from *Monster, fromPos geometry.Point, to *Monster) {
		if from.Squelch > to.Squelch {
			to.Last, to.Squelch = fromPos, from.Squelch-1
		}
		to.Dormant = false
	}
	if l := packLeader(m, mon); l != nil {
		lead := l.monster
		clear := lineNoSolid(m, a.MapPosition(), l.MapPosition())
		d := a.MapPosition().Sub(l.MapPosition())
		switch {
		case clear && mon.LeaderFlag == 2 && geometry.Abs(d.X) < 4 && geometry.Abs(d.Y) < 4:
			lead.PackSize++
			mon.LeaderFlag = 1
		case !clear && mon.LeaderFlag == 1:
			lead.PackSize--
			mon.LeaderFlag = 2
		}
		if mon.LeaderFlag == 1 {
			tell(mon, a.MapPosition(), lead)
		}
		return
	}
	if packRuled(mon) {
		for _, mi := range packMinions(m, a) {
			tell(mon, a.MapPosition(), mi.monster)
		}
	}
}

// lineNoSolid is LineClearF(CheckNoSolid): no wall between, monsters don't count.
func lineNoSolid(m *packMap, from, to geometry.Point) bool {
	los := geometry.LineOfSight(from, to, func(p geometry.Point) bool { return p == from || p == to || m.IsTileWalkable(p) })
	return los[len(los)-1] == to
}

// d1LeavePack ports MAI_Scav's break with its pack (packsize drops even for a strayed minion, as in 1.09).
func LeavePack(m *packMap, mon *Monster) {
	if l := packLeader(m, mon); l != nil {
		l.monster.PackSize--
	}
	mon.LeaderFlag = 0
}

// releasePack ports M_UpdateLeader for a monster that died: its minions are on their own, and a minion's leader
// has one fewer.
func releasePack(m *packMap, dead *Actor) {
	mon := dead.monster
	for _, a := range m.Actors() {
		if o := a.monster; o != nil && o.LeaderFlag == 1 && o.LeaderIdx == mon.SpawnIndex && mon.SpawnIndex >= 0 {
			o.LeaderFlag = 0
		}
	}
	if mon.LeaderFlag == 1 {
		if l := packLeader(m, mon); l != nil {
			l.monster.PackSize--
		}
	}
	mon.LeaderFlag = 0
}
