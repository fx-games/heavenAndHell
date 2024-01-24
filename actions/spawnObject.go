package actions

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
)

type SpawnAction struct {
	BaseAction
	spawnPos          geometry.Point
	spawnFunc         func(location geometry.Point)
	actionDescription string
}

func NewSpawnObjectAction(source Cause, spawnFunc func(location geometry.Point), spawnPos geometry.Point) *SpawnAction {
	return &SpawnAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameSpawnObject,
		},
		spawnFunc:         spawnFunc,
		spawnPos:          spawnPos,
		actionDescription: "walls of fire",
	}
}
func (d *SpawnAction) Execute(engine Engine) []Action {
	d.spawnFunc(d.spawnPos)

	creator := d.Cause().GetActor()
	if creator == nil {
		return nil
	}
	casterPart := fmt.Sprintf("%s creates", creator.Name())
	if creator == engine.GetWorldState().GetPlayer() {
		casterPart = "You create"
	}
	engine.Log(fmt.Sprintf("%s %s", casterPart, d.actionDescription))
	return nil
}

func (d *SpawnAction) GetAnimations() []common.Animation {
	return nil
}
