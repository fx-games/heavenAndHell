package common

import (
	"rpg/geometry"
	"rpg/gridmap"
)

// A pack is a unique with mUnqAttr & 2 and the minions PlaceGroup tied to it. D1MonsterState keeps the ties:
// LeaderIdx is the leader's SpawnIndex (its slot on the level), LeaderFlag 1 while a minion keeps up with it and 2
// once it strayed, PackSize on the leader the minions at flag 1.

type d1Map = gridmap.GridMap[*Actor, *Item, Object]

// d1PackLeader is a minion's leader, if it still has one.
func d1PackLeader(m *d1Map, mon *D1Monster) *Actor {
	if mon.LeaderFlag == 0 {
		return nil
	}
	for _, a := range m.Actors() {
		if a.d1Monster != nil && a.d1Monster.SpawnIndex == mon.LeaderIdx && a.IsAlive() {
			return a
		}
	}
	return nil
}

// d1PackMinions are a leader's minions at flag 1.
func d1PackMinions(m *d1Map, leader *Actor) (out []*Actor) {
	for _, a := range m.Actors() {
		if mon := a.d1Monster; mon != nil && mon.LeaderFlag == 1 && mon.LeaderIdx == leader.d1Monster.SpawnIndex && a != leader && a.IsAlive() {
			out = append(out, a)
		}
	}
	return
}

func d1PackRuled(mon *D1Monster) bool { return mon.Unique != nil && mon.Unique.UnqAttr&2 != 0 }

// D1PackStepOK is DirOK's pack half for a step onto p: a minion keeps within 3 cells of its leader, and a leader
// with mUnqAttr & 2 only moves when its whole pack stands within 3 cells of where it goes.
func D1PackStepOK(m *d1Map, a *Actor, p geometry.Point) bool {
	mon := a.d1Monster
	if mon.LeaderFlag == 1 {
		if l := d1PackLeader(m, mon); l != nil {
			d := p.Sub(l.MapPosition())
			return geometry.Abs(d.X) < 4 && geometry.Abs(d.Y) < 4
		}
		return true
	}
	if !d1PackRuled(mon) {
		return true
	}
	n := 0
	for _, mi := range d1PackMinions(m, a) {
		if d := mi.MapPosition().Sub(p); geometry.Abs(d.X) <= 3 && geometry.Abs(d.Y) <= 3 {
			n++
		}
	}
	return n == mon.PackSize
}

// D1GroupUnity ports GroupUnity, run after each thing a monster does: a minion out of its leader's sight strays
// (flag 2), one back in sight and within 3 cells rejoins; the more alert of leader and minion tells the other
// where the enemy was, and wakes a dormant gargoyle.
func D1GroupUnity(m *d1Map, a *Actor) {
	mon := a.d1Monster
	tell := func(from *D1Monster, fromPos geometry.Point, to *D1Monster) {
		if from.Squelch > to.Squelch {
			to.Last, to.Squelch = fromPos, from.Squelch-1
		}
		to.Dormant = false
	}
	if l := d1PackLeader(m, mon); l != nil {
		lead := l.d1Monster
		clear := d1LineNoSolid(m, a.MapPosition(), l.MapPosition())
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
	if d1PackRuled(mon) {
		for _, mi := range d1PackMinions(m, a) {
			tell(mon, a.MapPosition(), mi.d1Monster)
		}
	}
}

// d1LineNoSolid is LineClearF(CheckNoSolid): no wall between, monsters don't count.
func d1LineNoSolid(m *d1Map, from, to geometry.Point) bool {
	los := geometry.LineOfSight(from, to, func(p geometry.Point) bool { return p == from || p == to || m.IsTileWalkable(p) })
	return los[len(los)-1] == to
}

// d1LeavePack ports MAI_Scav's break with its pack (packsize drops even for a strayed minion, as in 1.09).
func D1LeavePack(m *d1Map, mon *D1Monster) {
	if l := d1PackLeader(m, mon); l != nil {
		l.d1Monster.PackSize--
	}
	mon.LeaderFlag = 0
}

// d1ReleasePack ports M_UpdateLeader for a monster that died: its minions are on their own, and a minion's leader
// has one fewer.
func d1ReleasePack(m *d1Map, dead *Actor) {
	mon := dead.d1Monster
	for _, a := range m.Actors() {
		if o := a.d1Monster; o != nil && o.LeaderFlag == 1 && o.LeaderIdx == mon.SpawnIndex && mon.SpawnIndex >= 0 {
			o.LeaderFlag = 0
		}
	}
	if mon.LeaderFlag == 1 {
		if l := d1PackLeader(m, mon); l != nil {
			l.d1Monster.PackSize--
		}
	}
	mon.LeaderFlag = 0
}
