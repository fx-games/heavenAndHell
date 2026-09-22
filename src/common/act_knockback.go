package common

import (
	"rpg/src/geometry"
	"rpg/src/stats"
	"rpg/src/util"
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
	animation         *MovementAnimation
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
		if d.isNotAnimated() {
			actor.SetDrawPosition(d.targetPos.ToPointF())
		}
	}

	if !gameMap.IsCurrentlyPassable(d.obstaclePos) {
		// problem.. this will trigger at the same tick as the knockback animation begins..
		actionPushed := NewChangeResourceAction(d.Cause(), actor, stats.Health, -5)
		actionPushed.SetAnimationStartPredicate(func() bool {
			return d.isNotAnimated() || d.animation.IsDone()
		})
		actions = append(actions, actionPushed)
		if gameMap.IsActorAt(d.obstaclePos) {
			hitActor := gameMap.ActorAt(d.obstaclePos)
			action := NewChangeResourceAction(d.Cause(), hitActor, stats.Health, -5)
			action.SetAnimationStartPredicate(func() bool {
				return d.isNotAnimated() || d.animation.IsDone()
			})
			actions = append(actions, action)
		}
	}
	d.prepareAnimation()
	//println(fmt.Sprintf("Actor %s moved to %s", actor.Action(), d.targetPos))
	return actions
}
func (d *KnockbackAction) GetAnimations() []Animation {
	if d.isNotAnimated() {
		return nil
	}
	d.animation.SetSimplePath(d.originPos, d.targetPos)
	return OneAnimation(d.animation)
	//return &DummyAnimation{time: 5, cancelRunningAnimations: true}
}

func (d *KnockbackAction) prepareAnimation() {
	// but really, the state of this movement animation should determine
	// when exactly the "walked on cell" effect animations trigger.
	realDistance := geometry.Distance(d.originPos, d.targetPos)
	ticksNeeded := util.SecondsToTicks(d.animationDelay * realDistance)
	d.animation = &MovementAnimation{
		actor:       d.actor,
		ticksNeeded: ticksNeeded,
		isForPlayer: false,
	}
}

func (d *KnockbackAction) isNotAnimated() bool {
	return d.animationDisabled || d.distance == 0
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
