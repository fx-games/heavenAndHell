package actions

import (
	"fmt"
	"rpg/common"
)

type UpdateFoVAction struct {
	BaseAction
	actor *common.Actor
}

func NewUpdateFoVAction(source Cause, actor *common.Actor) *UpdateFoVAction {
	return &UpdateFoVAction{
		BaseAction: BaseAction{
			cause: source,
			name:  "ActionNameUpdateFoV",
		},
		actor: actor,
	}
}
func (d *UpdateFoVAction) Execute(engine Engine) []Action {
	state := engine.GetWorldState()
	actor := d.actor
	currentMap := state.GetMap()

	visionRange := engine.GetPlayerVisionRange()
	currentMap.UpdateFieldOfView(actor.GetFOV(), actor.MapPosition(), visionRange)
	updatePlayerKnowledge(engine)

	if actor == state.GetPlayer() {
		d.updatePlayerDijkstra(engine)
	}
	return nil
}

func updatePlayerKnowledge(engine Engine) {
	state := engine.GetWorldState()
	player := state.GetPlayer()
	currentMap := state.GetMap()
	vis := state.GetVisibility()

	currentlyVisible := make(map[*common.Actor]bool)
	cameIntoView := make(map[*common.Actor]bool)
	wentOutOfView := make(map[*common.Actor]bool)

	for _, p := range player.VisionMap() {
		currentMap.SetExplored(p)
		if state.GetMap().IsActorAt(p) {
			visibleActor := state.GetMap().ActorAt(p)
			if visibleActor == player {
				continue
			}
			currentlyVisible[visibleActor] = true
			if !vis.IsVisible(visibleActor) {
				cameIntoView[visibleActor] = true
			}
		}
	}

	for previouslyVisible, _ := range vis.GetVisibleActors() {
		if _, exists := currentlyVisible[previouslyVisible]; !exists {
			wentOutOfView[previouslyVisible] = true
		}
	}
	vis.Set(currentlyVisible)
	for newActor, _ := range cameIntoView {
		if newActor.IsHostile() {
			engine.Log(fmt.Sprintf("%s comes into view", newActor.Name()))
		}
		if newActor.IsHostile() {
			newActor.SetState(common.ActorStateAttacking)
		}
		if newActor.HasSpeech(common.SpeechTriggerSight) {
			engine.Say(newActor, common.SpeechTriggerSight)
		}
	}
	for oldActor, _ := range wentOutOfView {
		if oldActor.IsHostile() {
			engine.Log(fmt.Sprintf("%s leaves your field of view", oldActor.Name()))
		}
	}
}

func (d *UpdateFoVAction) GetAnimations() []common.Animation {
	return nil
}

func (d *UpdateFoVAction) updatePlayerDijkstra(engine Engine) {
	state := engine.GetWorldState()
	actor := state.GetPlayer()
	currentMap := state.GetMap()
	dijkstraMap := currentMap.GetDijkstraMapWithActorsNotBlocking(actor.MapPosition(), 10000)
	engine.SetPlayerDijkstraMap(dijkstraMap)
}
