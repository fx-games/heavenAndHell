package common

import (
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"testing"
)

// DirOK's pack rules: a minion stays within 3 cells of its leader, a leader waits for its pack, and a death
// frees the minions (M_UpdateLeader).
func TestPack(t *testing.T) {
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](30, 30)
	mk := func(p geometry.Point, mon *Monster) *Actor {
		a := NewActor(0)
		a.stats.SetAttribute(stats.MaxHealth, 10)
		a.stats.GetResource(stats.Health).SetToMax()
		a.monster = mon
		m.AddActor(a, p)
		return a
	}
	leader := mk(geometry.Point{X: 10, Y: 10}, &Monster{SpawnIndex: 5, Unique: &rules.UniqueMonsterData{UnqAttr: 2},
		MonsterState: MonsterState{PackSize: 1}})
	minion := mk(geometry.Point{X: 12, Y: 10}, &Monster{SpawnIndex: 6, MonsterState: MonsterState{LeaderIdx: 5, LeaderFlag: 1}})
	if !PackStepOK(m, minion, geometry.Point{X: 13, Y: 10}) || PackStepOK(m, minion, geometry.Point{X: 14, Y: 10}) {
		t.Error("a minion may go 3 cells from its leader, not 4")
	}
	if !PackStepOK(m, leader, geometry.Point{X: 11, Y: 10}) || PackStepOK(m, leader, geometry.Point{X: 8, Y: 10}) {
		t.Error("the leader may only step where its pack stays within 3 cells")
	}
	releasePack(m, leader)
	if minion.monster.LeaderFlag != 0 || !PackStepOK(m, minion, geometry.Point{X: 20, Y: 10}) {
		t.Error("a dead leader's minions should be free")
	}
}
