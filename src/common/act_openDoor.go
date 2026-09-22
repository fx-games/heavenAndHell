package common

import (
	"rpg/src/geometry"
	"rpg/src/gridmap"
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

	if !playD1SfxAt(context, "IS_DOOROPEN", d.doorPos) { // OperateL1RDoor and kin
		context.AudioPlayer.PlayCue("open_door")
	}

	return actions
}

func (d *OpenDoorAction) GetAnimations() []Animation {
	return nil
}

// CloseDoorAction shuts an open door (OperateL1Door and kin on an open door). CanCloseDoorAt says when it may.
type CloseDoorAction struct {
	BaseAction
	actor   *Actor
	doorPos geometry.Point
}

func NewCloseDoorAction(source Cause, actor *Actor, doorPosition geometry.Point) *CloseDoorAction {
	return &CloseDoorAction{BaseAction: BaseAction{cause: source, name: ActionNameOpenDoor}, actor: actor, doorPos: doorPosition}
}

func (d *CloseDoorAction) GetDelayInAut() int         { return 10 }
func (d *CloseDoorAction) GetAnimations() []Animation { return nil }
func (d *CloseDoorAction) Execute(context ActionContext) []Action {
	o, _ := context.GameState.GetMap().TryGetObjectAt(d.doorPos)
	door, ok := o.(*Door)
	if !ok {
		return nil
	}
	door.isOpen = false
	if !playD1SfxAt(context, "IS_DOORCLOS", d.doorPos) {
		context.AudioPlayer.PlayCue("close_door")
	}
	return []Action{NewUpdateFoVAction(d.Cause(), d.actor)}
}

// CanCloseDoorAt: an open door next to the actor, with no one, no item and no corpse in the doorway.
func CanCloseDoorAt(m *gridmap.GridMap[*Actor, *Item, Object], actor *Actor, p geometry.Point) bool {
	o, _ := m.TryGetObjectAt(p)
	door, ok := o.(*Door)
	return ok && !door.IsClosed() && m.MoveDistance(actor.MapPosition(), p) == 1 &&
		!m.IsActorAt(p) && !m.IsItemAt(p) && !m.IsDownedActorAt(p)
}
