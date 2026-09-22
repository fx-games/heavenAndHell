package main

import (
	"image/color"
	"rpg/common"
	"rpg/geometry"
	"testing"
)

// The church's red light is a real light source on the town map, not reddish tiles: the cell at its door reads
// red and a cell well away from it does not.
func TestChurchGlowsRedInTown(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()
	town := state.GetMap()

	entrance := town.GetNamedLocation("to_cathedral")
	far := geometry.Point{X: entrance.X - 25, Y: entrance.Y + 10}
	if !town.Contains(entrance) || !town.Contains(far) {
		t.Fatalf("church at %v / reference cell at %v are off the town map", entrance, far)
	}
	town.SetExplored(entrance)
	town.SetExplored(far)

	white := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	lit := state.ApplyLighting(entrance, white, 0)
	plain := state.ApplyLighting(far, white, 0)
	if lit.R <= plain.R || lit.G >= plain.G {
		t.Fatalf("the church door draws as %v, a cell 25 tiles away as %v: no red light", lit, plain)
	}
}

// The town starts gloomy and brightens once the player has been underground.
func TestTownBrightensAfterFirstDescent(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()

	at := state.GetPlayer().MapPosition()
	state.GetMap().SetExplored(at)
	white := color.RGBA{R: 200, G: 200, B: 200, A: 255}

	gloomy := state.ApplyLighting(at, white, 0)
	state.GetFlags().SetTrue(common.FlagBeenUnderground)
	bright := state.ApplyLighting(at, white, 0)
	if bright.R <= gloomy.R || bright.G <= gloomy.G {
		t.Fatalf("first visit draws as %v, the return visit as %v: town does not brighten", gloomy, bright)
	}
}

// An open town portal lights the tiles right next to it light blue, and the light is gone once it closes.
func TestTownPortalGlowsBlue(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()
	town := state.GetMap()

	pos := town.GetNamedLocation("portal")
	next := geometry.Point{X: pos.X + 1, Y: pos.Y}
	far := geometry.Point{X: pos.X + 4, Y: pos.Y}
	for _, p := range []geometry.Point{pos, next, far} {
		town.SetExplored(p)
	}
	white := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	before := state.ApplyLighting(next, white, 0)
	farBefore := state.ApplyLighting(far, white, 0)

	state.SetD1Portal(&common.D1Portal{Map: "cathedral_1", Town: town.GetName(), X: 10, Y: 10})
	common.PlaceD1Portal(state, town)
	if lit := state.ApplyLighting(next, white, 0); lit.B <= before.B || lit.B <= lit.R {
		t.Fatalf("next to the portal: %v before, %v with it open: no blue light", before, lit)
	}
	if got := state.ApplyLighting(far, white, 0); got != farBefore {
		t.Fatalf("4 tiles from the portal the light changed from %v to %v: it reaches too far", farBefore, got)
	}

	state.CloseD1Portal()
	if got := state.ApplyLighting(next, white, 0); got != before {
		t.Fatalf("after closing the portal the cell next to it draws %v, was %v", got, before)
	}
}
