package common

import (
	"fmt"
	"rpg/src/stats"
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

	item, ok := gameMap.TryGetItemAt(actorPos)
	if !ok {
		return nil
	}

	gameMap.RemoveItem(item)

	playD1ItemInvSfx(context, item, actor)
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
