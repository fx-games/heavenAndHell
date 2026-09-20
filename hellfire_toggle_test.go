package main

import (
	"path/filepath"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/game"
	"rpg/gridmap"
	"testing"
)

// Hellfire off: no Hellfire quests or towners; switched on mid-game, the slot upgrades for good, but only in town.
func TestHellfireToggle(t *testing.T) {
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	theGame, ctx := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, false, "")
	state := theGame.GetWorldState()
	hasFarmer := func() bool {
		for _, a := range state.GetTownMap(string(defs.LevelNameTown)).Actors() {
			if a.GetVendorType() == common.VendorTypeFarmer {
				return true
			}
		}
		return false
	}
	if theGame.GetFactory().D1().D.Hellfire || state.QuestActive(d1.QFarmer) != d1.QuestNotAvail || hasFarmer() {
		t.Fatal("D1-only game has Hellfire content")
	}
	if game.ListSaveSlots()[0].Hellfire {
		t.Fatal("D1 slot listed as Hellfire")
	}
	state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(2), TargetLocation: "ladder_up"})
	if ctx.EnableHellfire(); theGame.IsHellfire() {
		t.Fatal("switched to Hellfire in the dungeon")
	}
	state.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: common.D1TownGates[1]})
	ctx.EnableHellfire() // DummyUI confirms
	state = theGame.GetWorldState()
	if !theGame.GetFactory().D1().D.Hellfire || state.QuestActive(d1.QFarmer) == d1.QuestNotAvail || !hasFarmer() {
		t.Fatal("D1 save didn't upgrade to Hellfire")
	}
	if err := theGame.LoadGame(); err != nil || !theGame.IsHellfire() || !game.ListSaveSlots()[0].Hellfire {
		t.Fatal("the upgrade didn't stick to the slot", err)
	}
}
