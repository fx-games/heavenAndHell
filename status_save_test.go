package main

import (
	"os"
	"path/filepath"
	"strings"
	"rpg/common"
	"rpg/defs"
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
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, true, "Aidan")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(1), TargetLocation: "ladder_up"})

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
	if err := theGame.LoadSlot(1); err != nil {
		t.Fatal(err)
	}

	state = theGame.GetWorldState()
	if got := state.GetPlayer().GetStatusEffects().AllEntries()[string(common.StatusInfravision)]; got != 5 {
		t.Fatalf("hero's infravision after load: %d turns, want 5", got)
	}
	for _, x := range state.GetMap().Actors() {
		if x.HasStatusEffect(common.StatusStone) {
			t.Logf("stoned at %v", x.MapPosition())
		}
	}
	b, _ := os.ReadFile(filepath.Join(game.SaveDir, "slot1.json"))
	t.Logf("stone in json: %v, map %s", strings.Count(string(b), "stone"), state.GetMap().GetName())
	a := state.GetMap().ActorAt(pos)
	if a == nil || a.GetStatusEffects().AllEntries()[string(common.StatusStone)] != 7 {
		t.Fatalf("monster at %v not stoned for 7 turns after load: %v", pos, a)
	}
}
