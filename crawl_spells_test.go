package main

import (
	"rpg/common"
	"rpg/d1"
	"rpg/geometry"
	"rpg/gridmap"
	"testing"
)

// Stone Curse and Teleport search 1.09's CrawlTable around the target: the curse takes a monster beside an empty
// target cell, and a teleport onto a monster lands on a free cell next to it.
func TestCrawlTableSpells(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m, player := s.GetMap(), s.GetPlayer()
	var mon *common.Actor
	var beside geometry.Point
	for _, a := range m.Actors() {
		for _, d := range common.D1Dirs {
			if p := a.MapPosition().Add(d); a != player && m.IsWalkable(p) && !m.IsActorAt(p) && !m.IsObjectAt(p) {
				mon, beside = a, p
			}
		}
	}
	if mon == nil {
		t.Fatal("no monster with a free neighbour on level 1")
	}
	var want *common.Actor // the first monster in 1.09's CrawlTable order around the target
	for _, ring := range d1.CrawlRings[:6] {
		for _, o := range ring {
			if a, ok := m.TryGetActorAt(beside.Add(geometry.Point{X: o[0], Y: o[1]})); ok && a != player && want == nil {
				want = a
			}
		}
	}
	g.GetActionBroker().Execute(common.NewSpellAction(player, d1.SplStone, 1, beside))
	if !want.HasStatusEffect(common.StatusStone) {
		t.Errorf("Stone Curse beside %v did not petrify the first monster in CrawlTable order, at %v", beside, want.MapPosition())
	}
	g.GetActionBroker().Execute(common.NewSpellAction(player, d1.SplTeleport, 1, mon.MapPosition()))
	if p := player.MapPosition(); p == mon.MapPosition() || geometry.DistanceChebyshev(p, mon.MapPosition()) > 2 {
		t.Errorf("Teleport onto the monster at %v landed on %v", mon.MapPosition(), p)
	}
}
