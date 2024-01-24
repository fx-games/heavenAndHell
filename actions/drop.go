package actions

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
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
	state := engine.GetWorldState()
	gameMap := state.GetMap() // read state
	actor := d.actor
	actorPos := actor.MapPosition()

	dropPos := d.getDropPos(gameMap, actorPos)

	inventory := actor.GetInventory()
	inventory.Remove(d.item)

	// TODO: currently possible to drop items out of bounds
	gameMap.AddItem(d.item, dropPos)

	name := actor.Name()
	if engine.GetWorldState().GetPlayer() == actor {
		d.item.DroppedByPlayer()
		name = "You"
	}
	engine.Log(fmt.Sprintf("%s dropped %s", name, d.item.Name()))

	return nil
}

func (d *DropAction) getDropPos(gameMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], actorPos geometry.Point) geometry.Point {
	isFree := func(pos geometry.Point) bool {
		return gameMap.Contains(pos) && gameMap.IsWalkable(pos) && !gameMap.IsItemAt(pos)
	}
	if isFree(actorPos) {
		return actorPos
	}
	freeCells := gameMap.GetFreeCellsForDistribution(actorPos, 1, isFree)
	dropPos := freeCells[0]
	return dropPos
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
