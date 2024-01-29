package actions

import (
	"fmt"
	"math/rand"
	"rpg/common"
	"rpg/geometry"
)

type MoveItemAction struct {
	BaseAction
	holder        *common.Actor
	item          common.Item
	targetPos     geometry.Point
	chanceToBreak float64
}
func NewMoveItemAction(source Cause, actor *common.Actor, targetPos geometry.Point, item common.Item) *MoveItemAction {
	return &MoveItemAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameMoveItem,
		},
		targetPos:      targetPos,
		holder:       actor,
		item: item,
	}
}

func (d *MoveItemAction) SetChanceToBreak(chance float64) {
	d.chanceToBreak = chance
}


func (d *MoveItemAction) GetDelayInAut() int {
	return 0
}

func (d *MoveItemAction) Execute(engine Engine) []Action {
	state := engine.GetWorldState()
	gameMap := state.GetMap() // read state

	inventory := d.holder.GetInventory()
	inventory.Remove(d.item)
	if rand.Float64() >= d.chanceToBreak {
		gameMap.AddItem(d.item, d.targetPos)
	} else {
		engine.Log(fmt.Sprintf("%s breaks!", d.item.Name()))
	}

	return nil
}

func (d *MoveItemAction) GetAnimations() []common.Animation {
	return nil
}

func (d *MoveItemAction) GetActor() *common.Actor {
	return d.holder
}
