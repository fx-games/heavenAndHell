package common

import (
	"fmt"

	"rpg/src/geometry"
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
	GetActor() *Actor
	GetNewPos() geometry.Point
}

func (d OnMoveHook) After(action Action, context ActionContext) []Action {
	var actions []Action
	state := context.GameState
	moveAction := action.(Mover)
	actor := moveAction.GetActor()
	newPos := moveAction.GetNewPos()
	currentMap := state.GetMap()

	actions = append(actions, TriggerCellEffectsDuringMovementForActor(state, action, actor, newPos)...)

	if actor != state.GetPlayer() {
		actions = append(actions, NewUpdateFoVAction(action.Cause(), actor))
		return actions
	}

	// player only
	transitionAtLocation, hasTransition := currentMap.GetTransitionAt(newPos)
	if hasTransition {
		actions = append(actions, NewTransitionAction(action.Cause(), transitionAtLocation))
	}

	// Walking past a secret door finds it half the time (Brogue searches passively as you go).
	for _, n := range currentMap.GetFilteredNeighbors(newPos, currentMap.Contains) {
		if currentMap.IsSecretDoorAt(n) && rng.Float64() < 0.5 {
			state.mapGen.revealSecretDoor(currentMap, n)
			context.Msg("You find a secret door.")
		}
	}
	actions = append(actions, []Action{
		NewUpdateFoVAction(action.Cause(), actor),
	}...)
	d.logObjectsAtLocation(context, newPos)
	return actions
}

func (d OnMoveHook) logObjectsAtLocation(context ActionContext, pos geometry.Point) *Item {
	state := context.GameState
	if item, ok := state.GetMap().TryGetItemAt(pos); ok {
		context.Msg(fmt.Sprintf("You see [:darkText]%s", item.Name()))
		return item
	}
	return nil
}

func TriggerCellEffectsDuringMovementForActor(state *State, sourceAction Action, actor *Actor, mapPos geometry.Point) []Action {
	if actor.HasStatusEffect(StatusFlying) {
		return nil
	}
	if object, ok := state.GetMap().TryGetObjectAt(mapPos); ok {
		object.ActorWalkedOn(actor)
	}
	return nil
}
