package main

import (
	"rpg/common"
	"rpg/defs"
	"rpg/gridmap"
	"testing"
)

// clicking the minimap to walk to the stairs used to leave the old level's path around, and the
// first turn on the (smaller) target map indexed its cells out of range
func TestAutoMovePathIsDroppedOnMapTransition(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[1], false, true, "")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(1), TargetLocation: "ladder_up"})

	player := state.GetPlayer()
	before := player.MapPosition()
	for _, stairs := range state.GetMap().GetAllStairs() {
		theGame.GetActions().StartPlayerAutoMovement(stairs)
		if player.MapPosition() != before {
			break
		}
	}
	if player.MapPosition() == before {
		t.Skip("no auto-move path on this seed")
	}

	state.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: common.D1TownGates[1]})
	state.PlayerTookTurn(10) // panicked here: stale path from level 1 indexed into the town map
	if !state.GetMap().Contains(player.MapPosition()) {
		t.Fatalf("player at %v is off the town map", player.MapPosition())
	}
}
