package actions

import (
	"fmt"
	"rpg/common"
)

type PickUpAction struct {
	BaseAction
	actor *common.Actor
}

func NewPickUpAction(source Cause, actor *common.Actor) *PickUpAction {
	return &PickUpAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNamePickup,
		},
		actor: actor,
	}
}
func (d *PickUpAction) Execute(engine Engine) []Action {
	state := engine.GetWorldState()
	gameMap := state.GetMap() // read state
	actor := d.actor
	actorPos := actor.MapPosition()

	if !gameMap.IsItemAt(actorPos) {
		return nil
	}
	item := gameMap.ItemAt(actorPos)

	gameMap.RemoveItem(item)

	inventory := actor.GetInventory()
	inventory.Add(item)

	name := actor.Name()
	if engine.GetWorldState().GetPlayer() == actor {
		name = "You"
	}
	engine.Log(fmt.Sprintf("%s picked up [:darkText]%s", name, item.Name()))

	return nil
}

func (d *PickUpAction) GetAnimations() []common.Animation {
	return nil
}
