package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"rpg/src/common"
	"rpg/src/game"
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

// Nothing but wall decorations (torches, hooked bodies, a lamp's light) is drawn over a D1 wall: no object, item
// or monster stands in one. And every cell has a name to hover (SceneryAt).
func TestNothingStandsInAWall(t *testing.T) {
	if testing.Short() {
		t.Skip("builds every D1 level")
	}
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	modOverride = "d1"
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	unnamed := map[string]int{}
	defer func() {
		for what, n := range unnamed {
			t.Errorf("%d cells have no name to hover: %s", n, what)
		}
	}()
	for seed := int64(1); seed <= 3; seed++ {
		theGame.SeedRandom(seed)
		os.RemoveAll(game.SaveDir)
		theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
		state := theGame.GetWorldState()
		for dlvl := 1; dlvl <= 16; dlvl++ {
			state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, dlvl), TargetLocation: "ladder_up"})
			m := state.GetMap()
			for y := 0; y < m.MapSize().Y; y++ {
				for x := 0; x < m.MapSize().X; x++ {
					p := geometry.Point{X: x, Y: y}
					if state.SceneryAt(p) == "" {
						unnamed[fmt.Sprintf("tile %d, decal %v", m.GetTileIconAt(p), m.Decals()[p])]++
					}
					if m.IsTileWalkable(p) {
						continue
					}
					if o, ok := m.TryGetObjectAt(p); ok {
						if _, prop := o.(*common.Prop); prop {
							t.Errorf("seed %d dlvl %d: %s stands in the wall at %v", seed, dlvl, o.ID(), p)
						}
					}
					if _, ok := m.TryGetActorAt(p); ok {
						t.Errorf("seed %d dlvl %d: a monster stands in the wall at %v", seed, dlvl, p)
					}
					if m.IsItemAt(p) {
						t.Errorf("seed %d dlvl %d: an item lies in the wall at %v", seed, dlvl, p)
					}
				}
			}
		}
	}
}
