package common

import (
	"rpg/gridmap"
)

type TransitionAction struct {
	BaseAction
	transitionTo gridmap.Transition
}

func (t TransitionAction) Execute(context ActionContext) []Action {
	context.GameState.MapTransition(t.transitionTo)
	return nil
}

func NewTransitionAction(source Cause, transition gridmap.Transition) *TransitionAction {
	return &TransitionAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameTransition,
		},
		transitionTo: transition,
	}
}
