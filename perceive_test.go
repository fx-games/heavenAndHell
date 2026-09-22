package main

import (
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"testing"
)

// Infravision (Empyrean Band, the spell, the Murky Pool) and clairvoyance draw monsters through walls, but hover
// text and right-click info used to ask CanSee - so those monsters had no name and no info window at all.
func TestInfravisionMonstersCanBeInspected(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[1], false, "")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, 1), TargetLocation: "ladder_up"})
	player := state.GetPlayer()

	var hidden *common.Actor
	for _, a := range state.GetMap().Actors() {
		if p := a.MapPosition(); !player.CanSee(p) && geometry.DistanceChebyshev(p, player.MapPosition()) <= 15 {
			hidden = a
			break
		}
	}
	if hidden == nil {
		t.Skip("no monster out of sight but in infravision range on this level")
	}
	pos := hidden.MapPosition()
	if player.CanPerceive(pos) {
		t.Fatalf("%s at %v is perceived without infravision", hidden.Name(), pos)
	}
	player.AddStatusEffect(common.StatusInfravision, 10)
	if !player.CanPerceive(pos) {
		t.Fatalf("infravision draws %s at %v but the player cannot inspect it", hidden.Name(), pos)
	}
	if far := player.MapPosition().Add(geometry.Point{X: 16, Y: 16}); !player.CanSee(far) && player.CanPerceive(far) {
		t.Fatalf("infravision reaches %v, 16 tiles from the player", far)
	}
}
