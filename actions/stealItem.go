package actions

import (
	"fmt"
	"math/rand"
	"rpg/common"
)

type StealItemAction struct {
	BaseAction
	actor  *common.Actor
	victim *common.Actor
}

func NewStealItemAction(source Cause, thief *common.Actor, victim *common.Actor) *StealItemAction {
	return &StealItemAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNamePickup,
		},
		actor: thief,
		victim: victim,
	}
}
func (d *StealItemAction) Execute(engine Engine) []Action {
	thief := d.actor

	victimItems := d.victim.GetStealableItems()

	if len(victimItems) == 0 {
		engine.Log(fmt.Sprintf("%s did not have items that can be stolen", d.victim.Name()))
		return nil
	}

	randomItem := victimItems[rand.Intn(len(victimItems))]

	victimInventory := d.victim.GetInventory()
	victimInventory.Remove(randomItem)

	inventory := thief.GetInventory()
	inventory.Add(randomItem)

	name := thief.Name()
	if engine.GetWorldState().GetPlayer() == thief {
		name = "You"
	}
	engine.Log(fmt.Sprintf("%s stole [:darkText]%s", name, randomItem.Name()))

	return nil
}

func (d *StealItemAction) GetAnimations() []common.Animation {
	return nil
}
