package main

import (
	"rpg/d1"
	"rpg/defs"
	"rpg/gridmap"
	"testing"
)

// Space recasts the last staff spell too, spending one of the staff's charges each time.
func TestRecastStaffSpell(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, true, "")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(1), TargetLocation: "ladder_up"}) // no spells in town
	player := state.GetPlayer()
	staff := theGame.GetFactory().NewD1Item(&d1.Item{Name: "Staff of Infravision", Type: d1.ITypeStaff, Spell: d1.SplInfra, Charges: 5, MaxCharges: 5})
	player.GetInventory().Add(staff)

	theGame.GetActions().GetContextActionsForItem(staff)[0].Action()
	theGame.GetActions().CastLastSpellAtNearest()
	if got := staff.D1().Charges; got != 3 {
		t.Fatalf("staff has %d charges after a cast and a recast, want 3", got)
	}
}
