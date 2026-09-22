package common

import (
	"rpg/src/geometry"
	"rpg/src/stats"
)

type KnockbackAction struct {
	BaseAction
	actor             *Actor
	animationDelay    float64
	animationDisabled bool
	direction         geometry.Point
	distance          int
	originPos         geometry.Point
	targetPos         geometry.Point
	obstaclePos       geometry.Point
}

func NewKnockbackAction(source Cause, actor *Actor, direction geometry.Point, distance int) *KnockbackAction {
	k := &KnockbackAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameMove,
		},
		actor:          actor,
		direction:      direction,
		distance:       distance,
		animationDelay: 0.05,
	}
	return k
}

func (d *KnockbackAction) SetAnimationDelay(delay float64) {
	d.animationDelay = delay
}

func (d *KnockbackAction) Execute(context ActionContext) []Action {

	state := context.GameState
	gameMap := state.GetMap() // read state
	actor := d.actor
	// calculate new position
	d.originPos = actor.MapPosition()
	destPos := d.originPos.Add(d.direction.Mul(d.distance + 1))
	los := gameMap.BresenhamLine(d.originPos, destPos, nil) // walls end it; IsCurrentlyPassable below stops at actors
	d.targetPos = d.originPos
	var actions []Action
	distanceMoved := 0
	for i, p := range los {
		pos := p
		if gameMap.IsCurrentlyPassable(pos) {

			distanceMoved++
			d.targetPos = pos

			gameMap.MoveActor(actor, pos) // write state

			startAnimating := func() bool {
				return actor.DrawPosition().ToPoint() == pos
			}

			// Should be an action
			actions = append(actions, TriggerCellEffectsDuringMovementForActor(state, d, actor, pos, startAnimating)...)

			if distanceMoved == d.distance {
				if i+1 < len(los) {
					d.obstaclePos = los[i+1]
				}
				break
			}
		} else {
			d.obstaclePos = pos
			break
		}
	}
	d.distance = distanceMoved
	if distanceMoved > 0 {
		gameMap.MoveActorFrom(actor, d.originPos, d.targetPos) // write state
	}
	context.ActionBroker.Emit(KnockedBack{Actor: actor, From: d.originPos, To: d.targetPos, Delay: d.animationDelay, Quiet: d.animationDisabled})
	landed := func() bool { return actor.DrawPosition().ToPoint() == d.targetPos } // the push is over

	if !gameMap.IsCurrentlyPassable(d.obstaclePos) {
		// problem.. this will trigger at the same tick as the knockback animation begins..
		actionPushed := NewChangeResourceAction(d.Cause(), actor, stats.Health, -5)
		actionPushed.SetAnimationStartPredicate(landed)
		actions = append(actions, actionPushed)
		if hitActor, ok := gameMap.TryGetActorAt(d.obstaclePos); ok {
			action := NewChangeResourceAction(d.Cause(), hitActor, stats.Health, -5)
			action.SetAnimationStartPredicate(landed)
			actions = append(actions, action)
		}
	}
	return actions
}

func (d *KnockbackAction) GetActor() *Actor {
	return d.actor
}

func (d *KnockbackAction) GetOldPos() geometry.Point {
	return d.originPos
}

func (d *KnockbackAction) GetNewPos() geometry.Point {
	return d.targetPos
}

func (d *KnockbackAction) DisableAnimation() {
	d.animationDisabled = true
}

// ponytail: TriggerCellEffectsDuringMovementForActor now lives in z_onMove.go on this
// branch; its damaging-tile branch was dropped there, not here.
