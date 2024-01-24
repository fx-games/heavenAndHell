package actions

import (
	"rpg/common"
	"rpg/geometry"
)

type OpenDoorAction struct {
	BaseAction
	actor   *common.Actor
	door    *common.Door
	doorPos geometry.Point
	delayInAut int
}

func (d *OpenDoorAction) GetDelayInAut() int {
	return d.delayInAut
}

func NewOpenDoorAction(source Cause, actor *common.Actor, doorPosition geometry.Point) *OpenDoorAction {
	return &OpenDoorAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameOpenDoor,
		},
		actor:   actor,
		doorPos: doorPosition,
		delayInAut: 10,
	}
}
func (d *OpenDoorAction) Execute(engine Engine) []Action {
	objectAt, exists := engine.GetWorldState().GetMap().TryGetObjectAt(d.doorPos)
	if !exists {
		println("ERR: no object at door position")
		return []Action{}
	}
	door, isDoor := objectAt.(*common.Door)
	if !isDoor {
		println("ERR: object at door position is not a door")
		return []Action{}
	}
	d.door = door
	d.door.Open()

	actions := []Action{
		NewUpdateFoVAction(d.Cause(), d.actor),
	}

	engine.GetAudio().PlayCue("open_door")

	return actions
}

func (d *OpenDoorAction) GetAnimations() []common.Animation {
	return nil
}
