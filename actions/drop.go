package actions

import (
	"fmt"
	"rpg/common"
)

type DropAction struct {
	BaseAction
	actor *common.Actor
	item  common.Item
}

func NewDropAction(source Cause, actor *common.Actor, item common.Item) *DropAction {
	return &DropAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameDrop,
		},
		actor: actor,
		item:  item,
	}
}
func (d *DropAction) Execute(engine Engine) []Action {
	//state := engine.GetWorldState()
	//gameMap := state.GetMap() // read state
	actor := d.actor
	actorPos := actor.MapPosition()

	inventory := actor.GetInventory()
	inventory.Remove(d.item)

	// TODO: currently possible to drop items out of bounds
	engine.AddItemToMapWithDistribution(d.item, actorPos)

	name := actor.Name()
	if engine.GetWorldState().GetPlayer() == actor {
		d.item.DroppedByPlayer()
		name = "You"
	}
	engine.Log(fmt.Sprintf("%s dropped %s", name, d.item.Name()))

	return nil
}

func (d *DropAction) GetAnimations() []common.Animation {
	return nil
}

func (d *DropAction) GetActor() *common.Actor {
	return d.actor
}

func (d *DropAction) GetItem() common.Item {
	return d.item
}
