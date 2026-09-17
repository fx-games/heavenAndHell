package main

import (
	"path/filepath"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/game"
	"testing"
)

// Hellfire off: no Hellfire quests or towners; switched on, Continue upgrades the D1 save.
func TestHellfireToggle(t *testing.T) {
	game.SaveFile = filepath.Join(t.TempDir(), "save.json")
	theGame, ctx := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	cfg := ctx.StartInfos.UserConfiguration
	cfg.Hellfire = false
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
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
	theGame.SaveGame()
	cfg.Hellfire = true
	if err := theGame.LoadGame(); err != nil {
		t.Fatal(err)
	}
	state = theGame.GetWorldState()
	if !theGame.GetFactory().D1().D.Hellfire || state.QuestActive(d1.QFarmer) == d1.QuestNotAvail || !hasFarmer() {
		t.Fatal("D1 save didn't upgrade to Hellfire")
	}
}
