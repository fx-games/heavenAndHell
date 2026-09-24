package app

import (
	"testing"

	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

// Under H&H's carried light the hero sees only the torch's reach, but monsters see the torch from afar (and hear
// the hero close by): one standing six cells away in the dark, in a clear line, must come for the hero.
func TestHHMonstersNoticeTheTorchFromTheDark(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.SeedRandom(1)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	player := s.GetPlayer()
	player.Hero().MaxHPBase += 100000 << 6
	common.SyncHero(player)
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m := s.GetMap()
	for _, a := range m.Actors() {
		if a.Monster() == nil || !a.IsAlive() || a.Monster().Data.AI == "" {
			continue
		}
		start := a.MapPosition()
		for _, d := range []geometry.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			spot := geometry.Point{X: start.X + 6*d.X, Y: start.Y + 6*d.Y}
			if !m.CanPlaceActorHere(spot) || !clearLine(m, start, spot) || player.CanSee(start) {
				continue
			}
			m.MoveActor(player, spot)
			g.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
			if player.CanSee(start) {
				continue // lit by the level: not the case under test
			}
			for i := 0; i < 20 && a.IsAlive(); i++ {
				g.GetActions().EndTurn(10)
				for j := 0; j < 50; j++ {
					g.GetActionBroker().Tick()
				}
				if geometry.DistanceChebyshev(a.MapPosition(), spot) < 6 {
					return
				}
			}
			t.Fatalf("%s six cells away in the dark never came for the torch in 20 turns", a.GetInternalName())
		}
	}
	t.Skip("no monster in the dark with a clear six-cell line on depth 1")
}
