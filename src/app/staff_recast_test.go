package app

import (
	"rpg/src/gridmap"
	"rpg/src/rules"
	"testing"
)

// Space recasts the last staff spell too, spending one of the staff's charges each time.
func TestRecastStaffSpell(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, 1), TargetLocation: "ladder_up"}) // no spells in town
	player := state.GetPlayer()
	staff := theGame.GetFactory().NewRolledItem(&rules.Item{Name: "Staff of Infravision", Type: rules.ITypeStaff, Spell: rules.SplInfra, Charges: 5, MaxCharges: 5})
	player.GetInventory().Add(staff)

	theGame.GetActions().GetContextActionsForItem(staff)[0].Action()
	theGame.GetActions().CastLastSpellAtNearest()
	if got := staff.Roll().Charges; got != 3 {
		t.Fatalf("staff has %d charges after a cast and a recast, want 3", got)
	}
}
