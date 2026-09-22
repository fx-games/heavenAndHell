package app

import (
	"rpg/src/d1"
	"testing"
)

// The character sheet's "+" is offered per attribute only while a point is left and the class maximum is not
// reached, so a player can never push an attribute past its cap.
func TestCharacterSheetPlusRespectsMaximum(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	player := theGame.GetWorldState().GetPlayer()

	for _, at := range player.D1SheetAttributes() {
		if at.CanRaise {
			t.Fatalf("%s can be raised with no stat points spent", at.Label)
		}
	}
	player.Hero().StatPts = 500 // more than any class can ever spend
	for _, at := range player.D1SheetAttributes() {
		if !at.CanRaise {
			t.Fatalf("%s cannot be raised although 500 points are unspent", at.Label)
		}
	}
	for i := 0; i < 500; i++ {
		attrs := player.D1SheetAttributes()
		if !attrs[0].CanRaise {
			break
		}
		attrs[0].Raise()
	}
	str := player.D1SheetAttributes()[0]
	if str.Base != str.Max || str.CanRaise {
		t.Fatalf("strength stopped at %d/%d (canRaise=%v)", str.Base, str.Max, str.CanRaise)
	}
}

// The rules that only exist in the damage formulas have to be readable somewhere: the character sheet spells out
// what the weapon in hand does to each kind of monster, and the class traits nothing else mentions.
func TestCharacterSheetExposesHiddenRules(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "") // the warrior starts with a sword
	state := theGame.GetWorldState()
	player := state.GetPlayer()

	rows := map[string]string{}
	for _, sec := range append(player.D1SheetSections(), state.D1ThreatSection()) {
		for _, row := range sec.Rows {
			rows[row[0]] = row[1]
		}
	}
	for _, want := range []string{"vs. the undead", "vs. animals", "Critical strike", "Missile damage bonus", "Threat"} {
		if _, ok := rows[want]; !ok {
			t.Errorf("the character sheet has no %q row: %v", want, rows)
		}
	}
	// a sword halves its damage against the undead, so that row must be below the plain Damage row
	if rows["vs. the undead"] == rows["Damage"] {
		t.Errorf("sword damage vs. the undead (%s) equals its plain damage (%s)", rows["vs. the undead"], rows["Damage"])
	}
	if rows["Threat"] != "Normal" {
		t.Errorf("threat is %q before Diablo is beaten", rows["Threat"])
	}
	state.SetQuestActive(d1.QDiablo, d1.QuestDone)
	if got := state.D1ThreatSection().Rows[0][1]; got == "Normal" {
		t.Errorf("threat is still %q after beating Diablo", got)
	}
}
