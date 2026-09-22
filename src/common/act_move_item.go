package common

import (
	"fmt"
	"rpg/src/geometry"
)

type MoveItemAction struct {
	BaseAction
	holder        *Actor
	item          *Item
	targetPos     geometry.Point
	chanceToBreak float64
}

func NewMoveItemAction(source Cause, actor *Actor, targetPos geometry.Point, item *Item) *MoveItemAction {
	return &MoveItemAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameMoveItem,
		},
		targetPos: targetPos,
		holder:    actor,
		item:      item,
	}
}

func (d *MoveItemAction) SetChanceToBreak(chance float64) {
	d.chanceToBreak = chance
}

func (d *MoveItemAction) GetDelayInAut() int {
	return 0
}

func (d *MoveItemAction) Execute(context ActionContext) []Action {
	state := context.GameState

	inventory := d.holder.GetInventory()
	inventory.Remove(d.item)
	if rng.Float64() >= d.chanceToBreak {
		state.AddItemToMapWithDistribution(d.item, d.targetPos) // a throw can end at a wall: land on the nearest free cell
	} else {
		context.Msg(fmt.Sprintf("%s breaks!", d.item.Name()))
	}

	return nil
}

func (d *MoveItemAction) GetActor() *Actor {
	return d.holder
}
