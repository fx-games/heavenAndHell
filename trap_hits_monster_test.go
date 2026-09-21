package main

import (
	"rpg/common"
	"rpg/defs"
	"rpg/gridmap"
	"rpg/stats"
	"testing"
)

// A trap's arrow hits the monster in its way (CheckMissileCol's sourceless half, MonsterTrapHit).
func TestTrapArrowHitsMonster(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(1), TargetLocation: "ladder_up"})
	m := s.GetMap()
	for _, a := range m.Actors() {
		if a == s.GetPlayer() {
			continue
		}
		for _, d := range common.D1Dirs {
			from := a.MapPosition().Sub(d).Sub(d)
			if !m.IsWalkable(from) || !m.IsWalkable(a.MapPosition().Sub(d)) || m.IsActorAt(a.MapPosition().Sub(d)) {
				continue
			}
			hp := a.GetStats().GetResource(stats.Health).Value()
			for try := 0; try < 30 && a.GetStats().GetResource(stats.Health).Value() == hp; try++ {
				g.GetActionBroker().Execute(common.NewTrapMissileAction(from, common.MisArrow, a.MapPosition()))
			}
			if a.GetStats().GetResource(stats.Health).Value() == hp {
				t.Fatalf("30 trap arrows through %s never hurt it", a.Name())
			}
			return
		}
	}
	t.Fatal("no monster to shoot at on level 1")
}
