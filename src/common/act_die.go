package common

import (
	"fmt"
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

type DieAction struct {
	BaseAction
	actor *Actor
}

func NewDieAction(source Cause, actor *Actor) *DieAction {
	return &DieAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameDie,
		},
		actor: actor,
	}
}
func (d *DieAction) Execute(context ActionContext) []Action {
	state := context.GameState
	gameMap := state.GetMap() // read state
	msg := context.Msg
	actor := d.actor
	if actor == state.GetPlayer() {
		msg(fmt.Sprintf("You die. CoD: %s", d.cause.ToString()))
		state.factory.PlayD1HeroSound(71)
		PlayUICue("player_death")
		PlayUICue("game_over")
		context.Show(ShowGameOver{})
	} else {
		if actor.VisualsPending() {
			state.dying = append(state.dying, actor)
		}
		gameMap.RemoveActor(actor)
		if actor.HasCorpse() {
			corpsePos := d.getCorpsePos(gameMap, actor.MapPosition())
			gameMap.AddDownedActor(actor, corpsePos) // write state
		}
		msg(fmt.Sprintf(actor.GetDeathStringFormat(), actor.Name()))
		//engine.UpdateMiniMap()
	}

	return nil
}
func (d *DieAction) getCorpsePos(gameMap *gridmap.GridMap[*Actor, *Item, Object], actorPos geometry.Point) geometry.Point {
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

func (d *DieAction) GetActor() *Actor {
	return d.actor
}
