package app

import (
	"image/color"
	"testing"

	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
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

// A carried light glows in town too, tinted and fading at its radius, without burning fuel there (H&H's mod.rec);
// taking it off leaves the town as it was.
func TestCarriedLightGlowsInTown(t *testing.T) {
	h := newHeavenAndHellGame(t, NewDummyUI())
	h.NewGame(h.GetGameData().ClassDefinitions[0], false, "")
	s := h.GetWorldState()
	if !s.IsPlayerInTown() {
		t.Fatal("the game does not start in town")
	}
	player := s.GetPlayer()
	h.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
	p := player.MapPosition()
	base := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	torch := player.GetEquipment().GetLightSource()
	lit := s.ApplyLighting(p, base, 0)
	far := s.ApplyLighting(p.Add(geometry.Point{X: 5}), base, 0)
	if s.LightBurns() {
		t.Error("the torch burns fuel in town")
	}
	player.GetEquipment().UnEquip(torch)
	dark := s.ApplyLighting(p, base, 0)
	farDark := s.ApplyLighting(p.Add(geometry.Point{X: 5}), base, 0)
	if lit == dark || lit.B >= lit.R {
		t.Errorf("the torch does not light its own cell in town: %v with it, %v without", lit, dark)
	}
	if far != farDark {
		t.Errorf("the torch reaches 5 cells in town: %v with it, %v without", far, farDark)
	}
}

func TestFlickerPatterns(t *testing.T) {
	if got := defs.FlickerPattern("0.5 1"); len(got) != 2 || got[0] != 0.5 {
		t.Errorf("a mod's own pattern: %v", got)
	}
	if len(defs.FlickerPattern("smooth")) == len(defs.FlickerPattern("fire")) {
		t.Error("smooth flickers like fire")
	}
}
