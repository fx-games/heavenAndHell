package main

import (
	"rpg/common"
	"rpg/defs"
	"rpg/gridmap"
	"testing"
)

// The right-click description is built from titled sections: a monster gets its own numbers and the matchup
// against the hero, the hero gets the character sheet plus their attributes. Empty sections would leave the
// window blank, and a missing Matchup would take the forecast off it.
func TestActorInfoSections(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "Aidan")
	state := theGame.GetWorldState()
	player := state.GetPlayer()

	title, sections := common.ActorInfoSections(state, player)
	if title != "Aidan" {
		t.Fatalf("the hero's window is titled %q", title)
	}
	if section(sections, "Character") == nil || section(sections, "Attributes") == nil {
		t.Fatalf("the hero's window has no Character/Attributes block: %v", titles(sections))
	}

	state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(1), TargetLocation: "ladder_up"})
	var monster *common.Actor
	for _, a := range state.GetMap().Actors() {
		if a != player && a.D1Monster() != nil && a.IsAlive() {
			monster = a
			break
		}
	}
	if monster == nil {
		t.Skip("no monster on this level")
	}
	_, sections = common.ActorInfoSections(state, monster)
	creature := section(sections, "Creature")
	if creature == nil || len(creature.Rows) < 4 {
		t.Fatalf("%s has no Creature block: %v", monster.Name(), titles(sections))
	}
	if section(sections, "Matchup") == nil {
		t.Fatalf("%s has no Matchup forecast: %v", monster.Name(), titles(sections))
	}
	for _, sec := range sections {
		if len(sec.Rows) == 0 && len(sec.Notes) == 0 {
			t.Fatalf("empty section %q would draw a bare heading", sec.Title)
		}
	}
}

func section(sections []common.D1SheetSection, title string) *common.D1SheetSection {
	for i := range sections {
		if sections[i].Title == title {
			return &sections[i]
		}
	}
	return nil
}

func titles(sections []common.D1SheetSection) []string {
	var out []string
	for _, sec := range sections {
		out = append(out, sec.Title)
	}
	return out
}
