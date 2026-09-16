package common

import (
	"fmt"
	"rpg/stats"
)

type PickUpAction struct {
	BaseAction
	actor *Actor
}

func NewPickUpAction(source Cause, actor *Actor) *PickUpAction {
	return &PickUpAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNamePickup,
		},
		actor: actor,
	}
}
func (d *PickUpAction) Execute(context ActionContext) []Action {
	state := context.GameState
	gameMap := state.GetMap() // read state
	actor := d.actor
	actorPos := actor.MapPosition()

	if !gameMap.IsItemAt(actorPos) {
		return nil
	}
	item := gameMap.ItemAt(actorPos)

	gameMap.RemoveItem(item)

	if item.IsGold() {
		actor.GetStats().IncrementBy(stats.Gold, item.d1.Value)
		context.Msg(fmt.Sprintf("You pick up %d gold", item.d1.Value))
		return nil
	}

	inventory := actor.GetInventory()
	inventory.Add(item)
	if item.d1 != nil && actor == state.GetPlayer() {
		state.factory.d1QuestItemPickedUp(item.d1)
	}

	name := actor.Name()
	if context.GameState.GetPlayer() == actor {
		name = "You"
	}
	context.Msg(fmt.Sprintf("%s picked up [:darkText]%s", name, item.Name()))

	return nil
}

func (d *PickUpAction) GetAnimations() []Animation {
	return nil
}
