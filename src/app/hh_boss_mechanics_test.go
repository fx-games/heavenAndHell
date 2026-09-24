package app

import (
	"testing"

	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
)

// H&H's set-piece bosses fight under D1 brains (enemies.rec's "Boss mechanic" comments): each must strike a hero
// standing next to it, and King Zedek must raise skeletons at a hero who keeps his distance.
func TestHHBossesFightWithTheirMechanics(t *testing.T) {
	if testing.Short() {
		t.Skip("walks to the bosses' levels")
	}
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.SeedRandom(1)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	player := s.GetPlayer()
	player.Hero().MaxHPBase += 100000 << 6
	common.SyncHero(player)
	hp := player.GetStats().GetResource(stats.Health)
	turns := func(n int, stop func() bool) {
		for i := 0; i < n && !stop(); i++ {
			hp.SetToMax()
			g.GetActions().EndTurn(10)
			for j := 0; j < 50; j++ {
				g.GetActionBroker().Tick()
			}
		}
	}
	find := func(id string) *common.Actor {
		for _, a := range s.GetMap().Actors() {
			if a.GetInternalName() == id && a.IsAlive() {
				return a
			}
		}
		return nil
	}
	standAt := func(p geometry.Point) {
		s.GetMap().MoveActor(player, p)
		g.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
	}

	for boss, level := range map[string]string{"meat_jack": "cathedral_2", "king_zedek": "catacombs_8",
		"lucifer": "hell_16", "rogue": "doom_10", "rogue_dog": "doom_10"} {
		s.MapTransition(gridmap.Transition{TargetMap: level, TargetLocation: "ladder_up"})
		b := find(boss)
		if b == nil {
			t.Errorf("no %s on %s", boss, level)
			continue
		}
		free := s.GetMap().GetFreeCardinalNeighbors(b.MapPosition())
		if len(free) == 0 {
			t.Errorf("%s is walled in", boss)
			continue
		}
		standAt(free[0])
		hurt := false
		turns(40, func() bool { hurt = hurt || !hp.IsAtMax(); return hurt })
		if !hurt {
			t.Errorf("%s never struck a hero next to it in 40 turns", boss)
		}
	}

	// inLine puts the hero n cells from a boss on an open straight line, with a map dump when there is none
	inLine := func(b *common.Actor, n int) {
		t.Helper()
		m, bp := s.GetMap(), b.MapPosition()
		for _, d := range []geometry.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: -1}} {
			clear := true
			for k := 1; k <= n; k++ {
				clear = clear && m.CanPlaceActorHere(geometry.Point{X: bp.X + d.X*k, Y: bp.Y + d.Y*k})
			}
			if clear {
				standAt(geometry.Point{X: bp.X + d.X*n, Y: bp.Y + d.Y*n})
				return
			}
		}
		for y := bp.Y - 5; y <= bp.Y+5; y++ {
			row := ""
			for x := bp.X - 8; x <= bp.X+8; x++ {
				switch q := (geometry.Point{X: x, Y: y}); {
				case q == bp:
					row += "B"
				case m.CanPlaceActorHere(q):
					row += "."
				default:
					row += "#"
				}
			}
			t.Log(row)
		}
		t.Fatalf("no open line of %d cells from %s", n, b.GetInternalName())
	}

	// King Zedek, from a few steps away: skeletons rise
	s.MapTransition(gridmap.Transition{TargetMap: "catacombs_8", TargetLocation: "ladder_up"})
	count := func() (n int) {
		for _, a := range s.GetMap().Actors() {
			if a.Monster() != nil && a.Monster().Data.Has("raisable") && a.IsAlive() {
				n++
			}
		}
		return
	}
	before := count()
	inLine(find("king_zedek"), 4)
	turns(60, func() bool { return count() > before })
	if count() <= before {
		t.Errorf("King Zedek raised no skeleton in 60 turns at a hero four cells away")
	}

	// Lucifer, from afar: Apocalypse finds the hero
	s.MapTransition(gridmap.Transition{TargetMap: "hell_16", TargetLocation: "ladder_up"})
	inLine(find("lucifer"), 4)
	hurt := false
	turns(60, func() bool { hurt = hurt || !hp.IsAtMax(); return hurt })
	if !hurt {
		t.Error("Lucifer never hurt a hero four cells away in 60 turns")
	}
}
