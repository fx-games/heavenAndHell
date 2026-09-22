package common

import (
	"rpg/src/d1"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
	"testing"
)

// DirOK's pack rules: a minion stays within 3 cells of its leader, a leader waits for its pack, and a death
// frees the minions (M_UpdateLeader).
func TestD1Pack(t *testing.T) {
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](30, 30)
	mk := func(p geometry.Point, mon *D1Monster) *Actor {
		a := NewActor(0)
		a.stats.SetAttribute(stats.MaxHealth, 10)
		a.stats.GetResource(stats.Health).SetToMax()
		a.d1Monster = mon
		m.AddActor(a, p)
		return a
	}
	leader := mk(geometry.Point{X: 10, Y: 10}, &D1Monster{SpawnIndex: 5, Unique: &d1.UniqueMonsterData{UnqAttr: 2},
		D1MonsterState: D1MonsterState{PackSize: 1}})
	minion := mk(geometry.Point{X: 12, Y: 10}, &D1Monster{SpawnIndex: 6, D1MonsterState: D1MonsterState{LeaderIdx: 5, LeaderFlag: 1}})
	if !D1PackStepOK(m, minion, geometry.Point{X: 13, Y: 10}) || D1PackStepOK(m, minion, geometry.Point{X: 14, Y: 10}) {
		t.Error("a minion may go 3 cells from its leader, not 4")
	}
	if !D1PackStepOK(m, leader, geometry.Point{X: 11, Y: 10}) || D1PackStepOK(m, leader, geometry.Point{X: 8, Y: 10}) {
		t.Error("the leader may only step where its pack stays within 3 cells")
	}
	d1ReleasePack(m, leader)
	if minion.d1Monster.LeaderFlag != 0 || !D1PackStepOK(m, minion, geometry.Point{X: 20, Y: 10}) {
		t.Error("a dead leader's minions should be free")
	}
}
