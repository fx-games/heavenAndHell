package main

import (
	"rpg/common"
	"rpg/stats"
	"testing"
)

// StartPlrHit: the hero flinches at a blow of at least clvl, or when forced (charge, knockback) unless already
// flinching, but never on a killing blow.
func TestD1StartPlrHit(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()
	player, f := state.GetPlayer(), state.Factory()
	player.Hero().Level = 5
	hp := player.GetStats().GetResource(stats.Health).Value()

	if common.D1StartPlrHit(f, player, 4<<6, 4, false); player.HitRecovery() != 0 {
		t.Fatal("a blow below clvl flinched")
	}
	if common.D1StartPlrHit(f, player, 5<<6, hp, false); player.HitRecovery() != 0 {
		t.Fatal("a killing blow flinched")
	}
	if common.D1StartPlrHit(f, player, 5<<6, 5, false); player.HitRecovery() == 0 {
		t.Fatal("a blow of clvl didn't flinch")
	}
	player.TakeHitRecovery()
	if common.D1StartPlrHit(f, player, 0, 0, true); player.HitRecovery() == 0 {
		t.Fatal("a forced flinch didn't flinch")
	}
}
