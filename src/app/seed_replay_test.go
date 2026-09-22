package app

import (
	"fmt"
	"testing"

	"rpg/src/gridmap"
	"rpg/src/stats"
)

// A seed replays a whole game: every roll comes from the one source SeedRandom pins, so two runs from the same
// seed walk, fight and loot identically.
func TestSameSeedSameGame(t *testing.T) {
	run := func() string {
		g := newHeavenAndHellGame(t, NewDummyUI())
		g.SeedRandom(7)
		g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
		state := g.GetWorldState()
		state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, 2), TargetLocation: "ladder_up"})
		g.GetActions().StartAutoExplore()
		for i := 0; i < 20000; i++ {
			g.GetActionBroker().Tick()
		}
		p := state.GetPlayer()
		return fmt.Sprintf("%v hp=%v xp=%v gold=%v actors=%d", p.MapPosition(), p.GetStats().GetAttribute(stats.Health),
			p.GetStats().GetAttribute(stats.Experience), p.GetStats().GetAttribute(stats.Gold), len(state.GetMap().Actors()))
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("same seed, different games:\n%s\n%s", a, b)
	}
}
