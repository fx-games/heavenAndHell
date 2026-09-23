package app

import (
	"fmt"
	"rpg/src/common"
	"rpg/src/gridmap"
	"testing"
)

// a skeleton raised by a barrel or sarcophagus the hero just opened must be a tab target right away
func TestSpawnedSkeletonIsVisibleEnemy(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[1], false, "")
	state := theGame.GetWorldState()
	player := state.GetPlayer()
	spawns := 0
	for dlvl := 1; dlvl <= 4 && spawns < 5; dlvl++ {
		state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, dlvl), TargetLocation: "ladder_up"})
		m := state.GetMap()
		for _, obj := range m.Objects() {
			o, ok := obj.(*common.Prop)
			if !ok || o.PreSpawnType < 0 {
				continue
			}
			free := m.GetFreeCardinalNeighbors(o.MapPosition())
			if len(free) == 0 {
				continue
			}
			m.MoveActor(player, free[0])
			player.SetDrawPosition(free[0].ToPointF())
			theGame.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
			before := len(m.Actors())
			o.ActorBumpedInto(player)
			if len(m.Actors()) == before {
				continue
			}
			spawned := m.Actors()[len(m.Actors())-1]
			spawns++
			fmt.Printf("dlvl %d: %s raised at %v, hero %v, in view=%v, tab target=%v\n", dlvl, spawned.Name(), spawned.MapPosition(),
				player.MapPosition(), player.CanSee(spawned.MapPosition()), state.GetVisibility().IsVisible(spawned))
			if player.CanSee(spawned.MapPosition()) && !state.GetVisibility().IsVisible(spawned) {
				t.Errorf("dlvl %d: %s in view but not a tab target", dlvl, spawned.Name())
			}
		}
	}
	if spawns == 0 {
		t.Fatal("no barrel or sarcophagus raised a skeleton")
	}
}
