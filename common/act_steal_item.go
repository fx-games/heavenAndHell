package common

import (
	"fmt"
	"math/rand"
)

type StealItemAction struct {
	BaseAction
	actor  *Actor
	victim *Actor
}

func NewStealItemAction(source Cause, thief *Actor, victim *Actor) *StealItemAction {
	return &StealItemAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNamePickup,
		},
		actor:  thief,
		victim: victim,
	}
}
func (d *StealItemAction) Execute(context ActionContext) []Action {
	thief := d.actor
	victimItems := d.victim.GetStealableItems()
	msg := context.Msg
	if len(victimItems) == 0 {
		msg(fmt.Sprintf("%s did not have items that can be stolen", d.victim.Name()))
		return nil
	}

	randomItem := victimItems[rand.Intn(len(victimItems))]

	victimInventory := d.victim.GetInventory()
	victimInventory.Remove(randomItem)

	inventory := thief.GetInventory()
	inventory.Add(randomItem)

	name := thief.Name()
	if context.GameState.GetPlayer() == thief {
		name = "You"
	}

	msg(fmt.Sprintf("%s stole [:darkText]%s", name, randomItem.Name()))

	return nil
}

func (d *StealItemAction) GetAnimations() []Animation {
	return nil
}
