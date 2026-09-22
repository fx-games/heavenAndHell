package app

import (
	"rpg/src/gridmap"
	"testing"
)

// In H&H's dark (Lighting: carried) the nearest unexplored cell is usually the neighbor. Starting auto explore
// toward it took the single-step shortcut, a manual move that cancels exploring: one step per key press.
// Every stop must say why (an enemy, a pickup, nothing left); a silent one is the bug.
func TestAutoExploreKeepsGoingInTheDark(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.SeedRandom(1)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	var last string
	g.SetLogHook(func(s string) {
		if s != "You start exploring" {
			last = s
		}
	})
	state := g.GetWorldState()
	for dlvl := 1; dlvl <= 10; dlvl++ {
		state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, dlvl), TargetLocation: "ladder_up"})
		player := state.GetPlayer()
		for starts := 0; starts < 50 && player.IsAlive(); starts++ {
			last = ""
			g.GetActions().StartAutoExplore()
			idle, pos := 0, player.MapPosition()
			for i := 0; i < 200000 && idle < 300; i++ {
				g.GetActionBroker().Tick()
				if player.MapPosition() == pos {
					idle++
				} else {
					idle, pos = 0, player.MapPosition()
				}
			}
			switch last {
			case "":
				t.Fatalf("dlvl %d: exploring stopped at %v with nothing to say why", dlvl, player.MapPosition())
			case "Nothing to explore", "Nothing left to explore":
				starts = 50
			}
		}
	}
}
