package main

import (
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
	"testing"
)

// menuUI captures the menus the NPCs open instead of drawing them.
type menuUI struct {
	*DummyUI
	lastMenu []util.MenuItem
	windows  [][]util.LabelText
}

func (m *menuUI) OpenMenu(items []util.MenuItem)         { m.lastMenu = items }
func (m *menuUI) OpenTextWindow(buffer []util.LabelText) { m.windows = append(m.windows, buffer) }

// Gillian only collects for the town once Diablo is dead, she asks 200 gold per explored dungeon level, and her
// donation puts every monster on those levels back on its feet - with D1's Nightmare numbers, since the hero has
// been through the game once.
func TestGillianDonationRespawnsMonsters(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	ui := &menuUI{DummyUI: NewDummyUI()}
	theGame.SetAndInitUserInterface(ui)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()

	gillian := func() *common.Actor {
		for _, a := range state.GetTownMap(string(defs.LevelNameTown)).Actors() {
			if a.GetVendorType() == common.VendorTypeBarmaid {
				return a
			}
		}
		return nil
	}()
	if gillian == nil {
		t.Fatal("no Gillian in town")
	}
	talk := func() []util.MenuItem {
		ui.lastMenu = nil
		theGame.GetActionBroker().Execute(common.NewTalkToAction(common.AbstractCause("test"), gillian))
		return ui.lastMenu
	}

	// two dungeon levels explored, Diablo still alive: nothing to donate to yet
	for _, dlvl := range []int{1, 2} {
		state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: "ladder_up"})
	}
	dungeon := state.GetMap()
	state.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: common.D1TownGates[1]})
	if levels := state.ExploredD1DungeonLevels(); levels != 2 {
		t.Fatalf("explored 2 dungeon levels, counted %d", levels)
	}
	for _, item := range talk() {
		if len(item.MainText) > 8 && item.MainText[:8] == "Give her" {
			t.Fatalf("Gillian collects before Diablo is beaten: %q", item.MainText)
		}
	}

	state.SetQuestActive(d1.QDiablo, d1.QuestDone)
	want := "Give her 400 gold." // 200 per explored dungeon level
	donate := itemNamed(talk(), want)
	if donate == nil {
		t.Fatalf("no %q after beating Diablo, menu was %v", want, ui.lastMenu)
	}

	// clear level 2 of everything alive, then let her put it back
	before := 0
	for _, a := range append([]*common.Actor(nil), dungeon.Actors()...) {
		if a.D1Monster() != nil {
			before++
			dungeon.RemoveActor(a)
		}
	}
	if before == 0 {
		t.Skip("no monsters generated on this level")
	}
	state.GetPlayer().GetStats().SetAttribute(stats.Gold, 1000)
	donate.Action()
	if len(ui.windows) == 0 {
		t.Fatal("Gillian said nothing about the donation")
	}
	if gold := state.GetPlayer().GetStats().GetAttribute(stats.Gold); gold != 600 {
		t.Fatalf("donation took %d gold, want 400", 1000-gold)
	}
	after := 0
	for _, a := range dungeon.Actors() {
		if a.D1Monster() != nil {
			after++
		}
	}
	if after < before {
		t.Fatalf("respawn put back %d of %d monsters", after, before)
	}
	// and they arrived on Nightmare footing: level is up by D1's own bonus
	for _, a := range dungeon.Actors() {
		if mon := a.D1Monster(); mon != nil {
			if mon.Level < d1.NightmareLevelBonus {
				t.Fatalf("%s respawned at level %d, below the Nightmare bonus", a.Name(), mon.Level)
			}
			break
		}
	}
}
