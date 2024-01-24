package actions

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
)

type OnMoveHook struct {
	BaseActionHook
	autoPickup bool
}

func (d OnMoveHook) IsBefore() bool {
	return false
}

func (d OnMoveHook) IsAfter() bool {
	return true
}

func (d OnMoveHook) Name() string {
	return "OnMoveHook"
}

func NewMainOnMoveHook(autoPickup bool) *OnMoveHook {
	return &OnMoveHook{
		BaseActionHook: BaseActionHook{
			hookedAction: ActionNameMove,
		},
		autoPickup: autoPickup,
	}
}

type Mover interface {
	GetActor() *common.Actor
	GetNewPos() geometry.Point
}

func (d OnMoveHook) After(action Action, engine Engine) []Action {
	var actions []Action
	state := engine.GetWorldState()
	moveAction := action.(Mover)
	actor := moveAction.GetActor()
	newPos := moveAction.GetNewPos()
	currentMap := state.GetMap()

	engine.TriggerCellEffectsDuringMovementForActor(action, actor, newPos, AnimateNow())

	if actor != state.GetPlayer() {
		actions = append(actions, NewUpdateFoVAction(action.Cause(), actor))
		return actions
	}

	// player only
	transitionAtLocation, hasTransition := currentMap.GetTransitionAt(newPos)
	if hasTransition {
		actions = append(actions, NewTransitionAction(action.Cause(), transitionAtLocation))
	}

	actions = append(actions, []Action{
		NewUpdateFoVAction(action.Cause(), actor),
	}...)
	d.logObjectsAtLocation(engine, newPos)
	return actions
}

func (d OnMoveHook) logObjectsAtLocation(engine Engine, pos geometry.Point) common.Item {
	state := engine.GetWorldState()
	if state.GetMap().IsItemAt(pos) {
		item := state.GetMap().ItemAt(pos)
		engine.Log(fmt.Sprintf("You see %s", item.Name()))
		return item
	}
	return nil
}