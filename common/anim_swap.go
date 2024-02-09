package common

import (
	"rpg/geometry"
	"rpg/util"
)

type SwapAnimation struct {
	animationOne *MovementAnimation
	animationTwo *MovementAnimation
}

func NewSwapAnimation(actorOne *Actor, actorTwo *Actor, animationDelay float64, isForPlayer bool) *SwapAnimation {
	ticksNeeded := util.SecondsToTicks(animationDelay)
	animOne := &MovementAnimation{
		actor:       actorOne,
		path:        []geometry.Point{actorOne.MapPosition(), actorTwo.MapPosition()},
		ticksNeeded: ticksNeeded,
		isForPlayer: isForPlayer,
	}
	animTwo := &MovementAnimation{
		actor:       actorTwo,
		path:        []geometry.Point{actorTwo.MapPosition(), actorOne.MapPosition()},
		ticksNeeded: ticksNeeded,
		isForPlayer: isForPlayer,
	}
	return &SwapAnimation{
		animationOne: animOne,
		animationTwo: animTwo,
	}
}
func (m *SwapAnimation) Cancel() {
	m.animationTwo.Cancel()
	m.animationOne.Cancel()
}
func (m *SwapAnimation) Draw(renderer TileRenderer) {

}

func (m *SwapAnimation) IsDone() bool {
	return m.animationOne.IsDone() && m.animationTwo.IsDone()
}

func (m *SwapAnimation) Tick() {
	if m.IsDone() {
		return
	}

	m.animationOne.Tick()
	m.animationTwo.Tick()
}
