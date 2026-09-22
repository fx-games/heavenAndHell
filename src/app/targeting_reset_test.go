package app

import (
	"rpg/src/common"
	"rpg/src/geometry"
	"testing"
)

type cancelCountUI struct {
	*DummyUI
	cancels int
}

func (u *cancelCountUI) CancelTargeting() { u.cancels++ }

// Any attack the player makes ends targeting mode, however it was started.
func TestPlayerAttackEndsTargeting(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	ui := &cancelCountUI{DummyUI: NewDummyUI()}
	theGame.SetAndInitUserInterface(ui)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	player := theGame.GetWorldState().GetPlayer()

	ui.cancels = 0
	target := player.MapPosition().Add(geometry.Point{X: 1})
	theGame.GetActionBroker().Execute(common.NewRangedAttackAction(common.NewActorCause("test", player), player, target))
	if ui.cancels == 0 {
		t.Fatal("a player attack left targeting mode on")
	}
}
