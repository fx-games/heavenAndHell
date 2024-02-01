package actions

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
)

type DieAction struct {
	BaseAction
	actor *common.Actor
}

func NewDieAction(source Cause, actor *common.Actor) *DieAction {
	return &DieAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameDie,
		},
		actor: actor,
	}
}
func (d *DieAction) Execute(engine Engine) []Action {
	state := engine.GetWorldState()
	gameMap := state.GetMap() // read state
	actor := d.actor
	if actor == state.GetPlayer() {
		engine.Log(fmt.Sprintf("You die. CoD: %s", d.cause.ToString()))
		engine.OnPlayerDied()
	} else {
		gameMap.RemoveActor(actor)
		if actor.HasCorpse() {
			corpsePos := d.getCorpsePos(gameMap, actor.MapPosition())
			gameMap.AddDownedActor(actor, corpsePos) // write state
		}
		engine.Log(fmt.Sprintf(actor.GetDeathStringFormat(), actor.Name()))
		//engine.UpdateMiniMap()
	}


	return nil
}
func (d *DieAction) getCorpsePos(gameMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], actorPos geometry.Point) geometry.Point {
	isFree := func(pos geometry.Point) bool {
		return gameMap.Contains(pos) && gameMap.IsWalkable(pos) && !gameMap.IsDownedActorAt(pos)
	}
	if isFree(actorPos) {
		return actorPos
	}
	freeCells := gameMap.GetFreeCellsForDistribution(actorPos, 1, isFree)
	if len(freeCells) == 0 {
		return actorPos
	}
	dropPos := freeCells[0]
	return dropPos
}
func (d *DieAction) GetAnimations() []common.Animation {
	return nil
}

func (d *DieAction) GetActor() *common.Actor {
	return d.actor
}
