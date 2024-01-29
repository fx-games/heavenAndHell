package actions

import (
	"rpg/common"
)

type SwapPositionsAction struct {
	BaseAction
	actorOne                  *common.Actor
	actorTwo                  *common.Actor
	isPlayer                  bool
	animationDelay            float64
	isMovementVisibleToPlayer bool
	animationDisabled         bool
}

func NewSwapPositionsAction(source Cause, actorOne, actorTwo *common.Actor, isPlayer bool) *SwapPositionsAction {
	return &SwapPositionsAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameSwap,
		},
		actorOne:       actorOne,
		actorTwo:       actorTwo,
		isPlayer:       isPlayer,
		animationDelay: 0.2,
	}
}

func (d *SwapPositionsAction) SetAnimationDelay(delay float64) {
	d.animationDelay = delay
}
func (d *SwapPositionsAction) Execute(engine Engine) []Action {
	state := engine.GetWorldState()
	gameMap := state.GetMap() // read state

	gameMap.SwapPositions(d.actorOne, d.actorTwo) // write state

	d.isMovementVisibleToPlayer = false
	if d.isPlayer {
		d.isMovementVisibleToPlayer = true
	} else {
		player := state.GetPlayer()
		d.isMovementVisibleToPlayer = player.CanSee(d.actorOne.MapPosition()) || player.CanSee(d.actorTwo.MapPosition())
	}
	if d.isNotAnimated() {
		d.actorOne.SetDrawPosition(d.actorOne.MapPosition().ToPointF())
		d.actorTwo.SetDrawPosition(d.actorTwo.MapPosition().ToPointF())
	}
	return nil
}

func (d *SwapPositionsAction) GetAnimations() []common.Animation {
	if d.isNotAnimated() {
		return nil
	}
	return OneAnimation(NewSwapAnimation(d.actorOne, d.actorTwo, d.animationDelay, d.isPlayer))
	//return &DummyAnimation{time: 5, cancelRunningAnimations: true}
}

func (d *SwapPositionsAction) isNotAnimated() bool {
	return !d.isMovementVisibleToPlayer || d.animationDisabled
}

func (d *SwapPositionsAction) GetActors() []*common.Actor {
	return []*common.Actor{d.actorOne, d.actorTwo}
}

func (d *SwapPositionsAction) DisableAnimation() {
	d.animationDisabled = true
}
