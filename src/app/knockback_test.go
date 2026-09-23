package app

import (
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
	"testing"
)

// A push into a wall hurts at once, but the blow shows only once the actor is drawn where it landed.
func TestKnockbackIntoWall(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m, broker, a := s.GetMap(), g.GetActionBroker(), s.GetPlayer()
	hp := a.GetStats().GetResource(stats.Health).Value()
	for y := 0; y < m.MapSize().Y; y++ {
		for x := 0; x < m.MapSize().X; x++ {
			from := geometry.Point{X: x, Y: y}
			for _, d := range geometry.Dirs {
				land := from.Add(d)
				if !m.IsCurrentlyPassable(from) || !m.IsCurrentlyPassable(land) || m.IsWalkable(land.Add(d)) {
					continue
				}
				m.MoveActor(a, from)
				a.SetDrawPosition(from.ToPointF())
				broker.Execute(common.NewKnockbackAction(common.AbstractCause("test"), a, d, 2))
				if a.MapPosition() != land || a.GetStats().GetResource(stats.Health).Value() != hp-5 {
					t.Fatalf("pushed from %v toward %v: at %v with %d hp, want %v with %d", from, d, a.MapPosition(),
						a.GetStats().GetResource(stats.Health).Value(), land, hp-5)
				}
				if a.ShownHealth() != hp {
					t.Fatalf("the wall's blow showed (%d hp) before the push was drawn", a.ShownHealth())
				}
				for i := 0; i < 200 && a.DrawPosition().ToPoint() != land; i++ {
					broker.Tick()
				}
				if a.ShownHealth() != hp-5 {
					t.Fatalf("drawn at %v, still shown with %d hp", a.DrawPosition(), a.ShownHealth())
				}
				return
			}
		}
	}
	t.Fatal("nowhere on level 1 to push the hero into a wall")
}
