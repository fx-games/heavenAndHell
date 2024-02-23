package common

import (
	"fmt"
	"rpg/geometry"
)

type SpawnAction struct {
	BaseAction
	spawnPos          geometry.Point
	spawnFunc         func()
	actionDescription string
}

func NewSpawnAction(source Cause, nameOfCreation string, spawnFunc func()) *SpawnAction {
	return &SpawnAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameSpawnObject,
		},
		spawnFunc:         spawnFunc,
		actionDescription: nameOfCreation,
	}
}
func (d *SpawnAction) Execute(context ActionContext) []Action {
	d.spawnFunc()

	creator := d.Cause().GetActor()
	if creator == nil {
		return nil
	}
	casterPart := fmt.Sprintf("%s creates", creator.Name())
	if creator == context.GameState.GetPlayer() {
		casterPart = "You create"
	}
	context.Msg(fmt.Sprintf("%s %s", casterPart, d.actionDescription))
	return nil
}

func (d *SpawnAction) GetAnimations() []Animation {
	return nil
}
