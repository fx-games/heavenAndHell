package main

import (
	"path/filepath"
	"rpg/common"
	"rpg/game"
	"rpg/gridmap"
	"testing"
)

// Status effects survive a save and load: a Stone Cursed monster stays stone, the hero keeps Infravision.
func TestStatusEffectsSurviveSaveLoad(t *testing.T) {
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	ui := &slotUI{DummyUI: NewDummyUI(), choose: 1}
	theGame.SetAndInitUserInterface(ui)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "Aidan")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, 1), TargetLocation: "ladder_up"})

	var stoned *common.Actor
	for _, a := range state.GetMap().Actors() {
		if a.D1Monster() != nil {
			stoned = a
			break
		}
	}
	if stoned == nil {
		t.Fatal("no monster on level 1")
	}
	pos := stoned.MapPosition()
	stoned.AddStatusEffect(common.StatusStone, 7)
	state.GetPlayer().AddStatusEffect(common.StatusInfravision, 5)
	theGame.SaveToSlot(1)

	// What the round trip has to preserve is whatever was serialised, not a constant: an actor that acts
	// while the slot is written legitimately ticks a turn off its own effects first (Actor.BeforeTurn).
	wantStone := stoned.GetStatusEffects().AllEntries()[string(common.StatusStone)]
	wantInfravision := state.GetPlayer().GetStatusEffects().AllEntries()[string(common.StatusInfravision)]
	if wantStone == 0 || wantInfravision == 0 {
		t.Fatalf("effects already gone before the load: stone %d, infravision %d", wantStone, wantInfravision)
	}

	if err := theGame.LoadSlot(1); err != nil {
		t.Fatal(err)
	}

	state = theGame.GetWorldState()
	if got := state.GetPlayer().GetStatusEffects().AllEntries()[string(common.StatusInfravision)]; got != wantInfravision {
		t.Fatalf("hero's infravision after load: %d turns, want %d", got, wantInfravision)
	}
	a := state.GetMap().ActorAt(pos)
	if a == nil {
		t.Fatalf("no monster at %v after load", pos)
	}
	if got := a.GetStatusEffects().AllEntries()[string(common.StatusStone)]; got != wantStone {
		t.Fatalf("monster at %v stoned for %d turns after load, want %d", pos, got, wantStone)
	}
}
