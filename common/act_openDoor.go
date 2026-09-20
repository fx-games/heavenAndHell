package common

import (
	"rpg/geometry"
)

type OpenDoorAction struct {
	BaseAction
	actor      *Actor
	door       *Door
	doorPos    geometry.Point
	delayInAut int
}

func (d *OpenDoorAction) GetDelayInAut() int {
	return d.delayInAut
}

func NewOpenDoorAction(source Cause, actor *Actor, doorPosition geometry.Point) *OpenDoorAction {
	return &OpenDoorAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameOpenDoor,
		},
		actor:      actor,
		doorPos:    doorPosition,
		delayInAut: 10,
	}
}
func (d *OpenDoorAction) Execute(context ActionContext) []Action {
	objectAt, exists := context.GameState.GetMap().TryGetObjectAt(d.doorPos)
	if !exists {
		println("ERR: no object at door position")
		return []Action{}
	}
	door, isDoor := objectAt.(*Door)
	if !isDoor {
		println("ERR: object at door position is not a door")
		return []Action{}
	}
	d.door = door
	d.door.Open()

	actions := []Action{
		NewUpdateFoVAction(d.Cause(), d.actor),
	}

	openSfx := "IS_DOOROPEN" // OperateL1RDoor and kin; the Crypt's doors are IS_CROPEN
	if d1DungeonLevel(context.GameState) >= 21 {
		openSfx = "IS_CROPEN"
	}
	if !playD1SfxAt(context, openSfx, d.doorPos) {
		context.AudioPlayer.PlayCue("open_door")
	}

	return actions
}

func (d *OpenDoorAction) GetAnimations() []Animation {
	return nil
}
