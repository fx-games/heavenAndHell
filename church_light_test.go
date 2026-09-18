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

	entrance := town.GetNamedLocation(common.D1TownGates[1])
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
