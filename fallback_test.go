package main

import (
	"testing"
)

// With no mod the engine still runs: one room, one weapon, one enemy. It is the floor under every mod.
func TestFallbackRoom(t *testing.T) {
	t.Skip("LEVELS.md step 5: no engine fallback game; the fallback ships no levels.rec")
	g := newModGame(t, "", NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	if s.GetMap().GetName() != "town" {
		t.Fatalf("starts on %s", s.GetMap().GetName())
	}
	armed := false
	for _, it := range s.GetPlayer().GetEquipment().AllItems() {
		armed = armed || it.IsWeapon()
	}
	if !armed {
		t.Error("the hero has no weapon")
	}
	enemies := 0
	for _, a := range s.GetMap().Actors() {
		if a != s.GetPlayer() && a.IsHostile() {
			enemies++
		}
	}
	if enemies != 1 {
		t.Errorf("%d enemies in the room, want 1", enemies)
	}
}
