package common

import (
	"fmt"
)

type DropAction struct {
	BaseAction
	actor *Actor
	item  *Item
}

func NewDropAction(source Cause, actor *Actor, item *Item) *DropAction {
	return &DropAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameDrop,
		},
		actor: actor,
		item:  item,
	}
}
func (d *DropAction) Execute(context ActionContext) []Action {
	//state := context.GameState
	//gameMap := state.GetMap() // read state
	actor := d.actor
	actorPos := actor.MapPosition()

	inventory := actor.GetInventory()
	inventory.Remove(d.item)

	// TODO: currently possible to drop items out of bounds
	context.GameState.AddItemToMapWithDistribution(d.item, actorPos)

	name := actor.Name()
	if context.GameState.GetPlayer() == actor {
		d.item.DroppedByPlayer()
		name = "You"
	}
	context.Msg(fmt.Sprintf("%s dropped %s", name, d.item.Name()))

	return nil
}

func (d *DropAction) GetAnimations() []Animation {
	return nil
}

func (d *DropAction) GetActor() *Actor {
	return d.actor
}

func (d *DropAction) GetItem() *Item {
	return d.item
}
