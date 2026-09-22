package main

import (
	"testing"

	"rpg/gridmap"
)

// Each mod lights the dark its own way (mod.rec's Lighting): Diablo's hero carries a permanent light whose radius
// only the gear's affixes change; Heaven & Hell's hero sees only by the light source they carry, while it burns.
func TestLightingIsTheMods(t *testing.T) {
	g, _ := createGameAndUIContext() // d1
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	if got, want := s.PlayerLightRadius(), s.GetPlayer().HeroSheet().LightRad; got != want || got < 2 {
		t.Errorf("D1: light radius %d, want the hero's own %d with no light source", got, want)
	}

	h := newHeavenAndHellGame(t, NewDummyUI())
	h.NewGame(h.GetGameData().ClassDefinitions[0], false, "")
	s = h.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	player := s.GetPlayer()
	torch := player.GetEquipment().GetLightSource()
	if torch == nil {
		t.Fatal("H&H: the hero starts without a torch")
	}
	lit := s.PlayerLightRadius()
	if lit < 1 || lit >= player.HeroSheet().LightRad {
		t.Errorf("H&H: light radius %d with a torch; want the torch's, not Diablo's %d", lit, player.HeroSheet().LightRad)
	}
	player.GetEquipment().UnEquip(torch)
	if got := s.PlayerLightRadius(); got != 0 {
		t.Errorf("H&H: light radius %d without a light source, want 0", got)
	}
	if !s.GetMap().IsTileLit(player.MapPosition()) && s.GetPlayerVisionRange() != 0 {
		t.Errorf("H&H: sees %d cells in the dark without a light", s.GetPlayerVisionRange())
	}
}
